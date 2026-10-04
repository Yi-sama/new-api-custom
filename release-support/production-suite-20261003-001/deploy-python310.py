import argparse
import copy
import fcntl
import hashlib
import json
import os
from pathlib import Path
import secrets
import shutil
import socket
import subprocess
import time
from urllib.parse import urlsplit

from nginx_validation import validate_nginx

ROOT = Path("/opt/us-whitelist")
RELEASE = ROOT / "releases/production-suite-20261003-001"
BACKUP = ROOT / "backups/production-suite-20261003-001"
BASELINE = ROOT / "releases/studio-credit-20261003-2120"
NGINX = Path("/etc/nginx/conf.d/api-yisama.conf")
RUNTIME = ROOT / "runtime/tavern-production"
BASE = ["docker", "compose", "-p", "us-whitelist", "-f", str(ROOT / "docker-compose.yml"),
        "-f", str(BASELINE / "backend.override.json")]
NEXT = BASE + ["-f", str(RELEASE / "backend.override.json")]
TAVERN = ["docker", "compose", "-p", "us-whitelist-tavern-production",
          "-f", str(RELEASE / "tavern.compose.json")]
MANIFEST = json.loads((RELEASE / "candidate-manifest.json").read_text())


def run(args, payload=None, timeout=120, **kwargs):
    result = subprocess.run(args, input=payload, timeout=timeout,
                            stdout=kwargs.pop("stdout", subprocess.PIPE),
                            stderr=subprocess.PIPE, **kwargs)
    if result.returncode:
        if BACKUP.exists():
            with (BACKUP / "command-errors.log").open("ab") as log:
                log.write(result.stderr)
        raise RuntimeError(f"{args[0]} exited {result.returncode}; see private command-errors.log")
    return result.stdout or b""


def inspect(name):
    return json.loads(run(["docker", "inspect", name]))[0]


def save(path, data):
    path.write_text(json.dumps(data, indent=2) + "\n")


def digest(path):
    result = hashlib.sha256()
    with path.open("rb") as stream:
        for chunk in iter(lambda: stream.read(1024 * 1024), b""):
            result.update(chunk)
    return result.hexdigest()


def psql(sql):
    return run(["docker", "exec", "-i", "sub2api-postgres", "sh", "-c",
                'exec psql -X -At -v ON_ERROR_STOP=1 -U "$POSTGRES_USER" -d newapi'],
               sql.encode()).decode().strip()


def fetch(url, expected=200, host=None):
    args = ["curl", "-sS", "--max-time", "20"]
    if host:
        args += ["-H", "Host: " + host]
    output = run(args + ["-w", "\n%{http_code}", url])
    body, code = output.rsplit(b"\n", 1)
    if int(code) != expected:
        raise RuntimeError(f"HTTP {code.decode()} for {url}, expected {expected}")
    return body


def identity(container):
    return (container["Id"], container["State"]["StartedAt"])


def resources():
    mem = dict(line.split(":", 1) for line in Path("/proc/meminfo").read_text().splitlines())
    available = int(mem["MemAvailable"].split()[0]) // 1024
    free = shutil.disk_usage(ROOT).free // 1024 // 1024
    assert available >= 1500 and free >= 2000, "Insufficient release headroom"
    return {"available_ram_mib": available, "root_free_mib": free}


def validate_compose():
    before = json.loads(run(BASE + ["config", "--format", "json"]))
    after = json.loads(run(NEXT + ["config", "--format", "json"]))
    delta = json.loads((RELEASE / "backend.override.json").read_text())["services"]["new-api"]
    original, current = copy.deepcopy(before), copy.deepcopy(after)
    for config in (original, current):
        service = config["services"]["new-api"]
        service["environment"] = {k: v for k, v in service["environment"].items()
                                  if k not in delta["environment"]}
        service["volumes"] = [v for v in service["volumes"]
                               if v["target"] not in {m["target"] for m in delta["volumes"]}]
    assert original == current, "Unexpected production Compose delta"
    service = after["services"]["new-api"]
    assert all(service["environment"][k] == v for k, v in delta["environment"].items())
    mounts = {v["target"]: v for v in service["volumes"]}
    for expected in delta["volumes"]:
        actual = mounts.get(expected["target"], {})
        assert all(actual.get(key) == expected[key]
                   for key in ("type", "source", "target", "read_only")), "Release mount mismatch"
    assert before["services"]["new-api"]["image"] == after["services"]["new-api"]["image"]
    return before, after


def verify_files():
    for name, key in [("new-api", "backend_sha256"), ("suite-schema", "schema_tool_sha256"),
                      ("risuai-managed/server.cjs", "risu_runtime_bundle_sha256")]:
        assert digest(RELEASE / name) == MANIFEST[key], "Release binary hash mismatch"
    for line in (RELEASE / "SHA256SUMS").read_text().splitlines():
        expected, name = line.split("  ", 1)
        path = (RELEASE / name).resolve()
        assert path.is_relative_to(RELEASE) and digest(path) == expected, "Package hash mismatch"
    html = (RELEASE / "risuai-managed/dist/index.html").read_text()
    assert "/tavern-app/assets/" in html
    assert MANIFEST["legal_configuration_deferred"] is True


def prepare():
    resources()
    verify_files()
    main = inspect("new-api")
    assert run(["docker", "exec", "new-api", "sha256sum", "/new-api"]).decode().split()[0] == MANIFEST["production_baseline_sha256"]
    assert main["Config"]["Labels"]["com.docker.compose.project.config_files"].split(",") == [BASE[5], BASE[7]]
    env = dict(v.split("=", 1) for v in main["Config"]["Env"])
    dsn = urlsplit(env["SQL_DSN"])
    assert dsn.hostname == "sub2api-postgres" and dsn.path == "/newapi" and dsn.username == "newapi"
    assert not env.get("LOG_SQL_DSN") or env["LOG_SQL_DSN"] == env["SQL_DSN"], "Separate log DB needs its own backup"
    assert env.get("NODE_TYPE") == "slave"
    assert not BACKUP.exists(), "Use existing verified backup; do not overwrite"
    assert not RUNTIME.exists(), "Production Tavern runtime already exists; reconcile first"
    with socket.socket() as listener:
        listener.bind(("127.0.0.1", 32831))
    before, after = validate_compose()
    BACKUP.mkdir(mode=0o700)
    save(BACKUP / "compose.before.json", before)
    save(BACKUP / "compose.after.json", after)
    running = run(["docker", "ps", "-q"]).decode().split()
    containers = json.loads(run(["docker", "inspect"] + running))
    save(BACKUP / "containers.before.json", containers)
    for index, name in enumerate(main["Config"]["Labels"]["com.docker.compose.project.config_files"].split(",")):
        shutil.copy2(name, BACKUP / f"compose-{index}.original")
    shutil.copy2(NGINX, BACKUP / "api-yisama.conf")
    assert not NGINX.is_symlink(), "Original ingress type changed; reconcile rollback"
    with (BACKUP / "newapi.dump").open("wb") as output:
        run(["docker", "exec", "sub2api-postgres", "sh", "-c",
             'exec pg_dump -U "$POSTGRES_USER" -d newapi -Fc'], stdout=output, timeout=240)
    with (BACKUP / "newapi.dump").open("rb") as source:
        toc = run(["docker", "exec", "-i", "sub2api-postgres", "pg_restore", "--list"], stdin=source)
    (BACKUP / "newapi.toc").write_bytes(toc)
    with (BACKUP / "newapi.dump").open("rb") as source:
        run(["docker", "exec", "-i", "sub2api-postgres", "pg_restore", "--file=/dev/null"],
            stdin=source, timeout=240)
    assert b"image_studio_credits" in toc and b"TABLE DATA" in toc
    mounts = [m for m in main["Mounts"] if m["Destination"] in {"/data", "/app/data", "/app/logs"}]
    assert len(mounts) == 3
    for index, mount in enumerate(mounts):
        path = Path(mount["Source"])
        assert path.is_dir() and str(path).startswith("/opt/")
        archive = BACKUP / f"data-{index}.tgz"
        run(["tar", "-czf", str(archive), "-C", str(path), "."], timeout=240)
        run(["tar", "-tzf", str(archive)], timeout=120)
    (BACKUP / "existing-data.before.txt").write_text(psql("""
BEGIN READ ONLY;
SELECT count(*) FROM users;
SELECT count(*) FROM channels;
SELECT md5(coalesce(json_agg(row_to_json(c) ORDER BY user_id)::text,'')) FROM image_studio_credits c;
SELECT status,count(*) FROM tasks WHERE status NOT IN ('SUCCESS','FAILURE') GROUP BY status;
COMMIT;
"""))
    RUNTIME.mkdir(parents=True, mode=0o700)
    (RUNTIME / "data").mkdir(mode=0o700)
    os.chown(RUNTIME / "data", 1000, 1000)
    with (RUNTIME / "tavern-service.key").open("x") as key:
        key.write(secrets.token_hex(32) + "\n")
    os.chown(RUNTIME / "tavern-service.key", 0, 1000)
    (RUNTIME / "tavern-service.key").chmod(0o440)
    original = NGINX.read_text()
    anchor = "    location / {"
    assert original.count(anchor) == 1
    addition = f"    include {RELEASE}/tavern-locations.nginx.conf;\n\n"
    (RELEASE / "api-yisama.conf").write_text(original.replace(anchor, addition + anchor, 1))
    validate_nginx(f"include {RELEASE / 'api-yisama.conf'};")
    shutil.copy2(RELEASE / "rollback.sh", BACKUP / "rollback.sh")
    (BACKUP / "rollback.sh").chmod(0o700)
    sums = {p.name: digest(p) for p in BACKUP.iterdir() if p.suffix in {".dump", ".tgz"}}
    state = {"status": "verified", "main_id": main["Id"], "original_ingress_sha256": digest(NGINX),
             "backup_hashes": sums, "data_mounts": mounts, "resources": resources()}
    save(BACKUP / "prepared.json", state)
    print("PRODUCTION_BACKUP_AND_CONFIG_VERIFIED", flush=True)


def wait_healthy(name):
    for _ in range(120):
        container = inspect(name)
        if not container["State"]["Running"] or container["RestartCount"] != 0:
            raise RuntimeError(name + " stopped or restarted")
        if container["State"].get("Health", {}).get("Status") == "healthy":
            return container
        time.sleep(2)
    raise RuntimeError(name + " did not become healthy")


def verify_runtime():
    main = wait_healthy("new-api")
    wait_healthy("risuai-production-20261003")
    assert run(["docker", "exec", "new-api", "sha256sum", "/new-api"]).decode().split()[0] == MANIFEST["backend_sha256"]
    env = dict(v.split("=", 1) for v in main["Config"]["Env"])
    assert env["NODE_TYPE"] == "slave" and env["UPDATE_TASK"] == "true"
    assert env["SYSTEM_TASK_TYPES"] == "video_studio_jobs,async_task_poll"
    status = json.loads(fetch("http://127.0.0.1:32794/api/status"))
    assert status["success"] and status["data"]["version"] == MANIFEST["version"]
    assert b"globalThis.__NODE__" in fetch("http://127.0.0.1:32831/", host="api.yisama.asia")
    for route in ["/api/video-studio/jobs", "/api/profitability/summary", "/api/image-studio/billing"]:
        fetch("http://127.0.0.1:32794" + route, 401)
    fetch("http://127.0.0.1:32831/api/new-api/account", 401, "api.yisama.asia")
    video = fetch("http://127.0.0.1:32794/video-canvas/video?v=2")
    assert b"/video-canvas/assets/" in video
    for asset in (RELEASE / "video-canvas/assets").iterdir():
        if asset.is_file():
            served = fetch("http://127.0.0.1:32794/video-canvas/assets/" + asset.name)
            assert hashlib.sha256(served).hexdigest() == digest(asset)
    before = {c["Name"]: c for c in json.loads((BACKUP / "containers.before.json").read_text())}
    running = run(["docker", "ps", "-q"]).decode().split()
    after = {c["Name"]: c for c in json.loads(run(["docker", "inspect"] + running))}
    changed = [name for name, c in before.items() if name != "/new-api" and
               (name not in after or identity(c) != identity(after[name]))]
    for name in ["/image-studio-open-webui-1", "/image-studio-gateway-1"]:
        assert name not in changed, "Production workstation container changed"
    save(BACKUP / "containers.after.json", list(after.values()))
    return {"status": "verified", "version": status["data"]["version"],
            "changed_other_containers": changed, "workstation_unchanged": True,
            "legal_configured": False, "tavern_native_notice_retained": True,
            "backup": str(BACKUP), "rollback": str(BACKUP / "rollback.sh"),
            "paid_calls": 0, "resources": resources()}


def activate():
    state = json.loads((BACKUP / "prepared.json").read_text())
    assert state["status"] == "verified"
    assert inspect("new-api")["Id"] == state["main_id"], "Concurrent production change"
    assert digest(NGINX) == state["original_ingress_sha256"], "Concurrent ingress change"
    resources()
    verify_files()
    validate_compose()
    for name, expected in state["backup_hashes"].items():
        assert digest(BACKUP / name) == expected, "Verified backup changed"
    schema = "/tmp/production-suite-schema-20261003-001"
    run(["docker", "cp", str(RELEASE / "suite-schema"), "new-api:" + schema])
    run(["docker", "exec", "new-api", schema, "-apply"])
    run(["docker", "exec", "new-api", schema])
    ownership = psql("""
BEGIN READ ONLY;
SELECT count(*) FROM pg_tables WHERE schemaname='public'
 AND tablename IN ('studio_jobs','tavern_credits') AND tableowner='newapi';
COMMIT;
""")
    assert "\n2\n" in "\n" + ownership + "\n", "New tables have unexpected ownership"
    print("ADDITIVE_SCHEMA_AND_APPLICATION_OWNERSHIP_VERIFIED", flush=True)
    switched = False
    try:
        switched = True
        run(NEXT + ["up", "-d", "--no-deps", "--no-build", "--pull", "never", "new-api"], timeout=180)
        wait_healthy("new-api")
        run(TAVERN + ["up", "-d", "--no-build", "--pull", "never", "risuai"], timeout=120)
        wait_healthy("risuai-production-20261003")
        temporary = NGINX.with_name("api-yisama.conf.production-suite-next")
        assert not temporary.exists() and not temporary.is_symlink()
        temporary.symlink_to(RELEASE / "api-yisama.conf")
        os.replace(temporary, NGINX)
        run(["nginx", "-t"])
        run(["systemctl", "reload", "nginx"])
        result = verify_runtime()
        save(BACKUP / "deployment-result.json", result)
        print("ACTIVATION_VERIFIED", flush=True)
        print(json.dumps(result), flush=True)
    except Exception:
        if switched:
            print("Activation failed; restoring applications/config only", flush=True)
            run(["sh", str(BACKUP / "rollback.sh"), "--allow-interruption"], timeout=240)
        raise


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("phase", choices=["prepare", "activate", "verify"])
    parser.add_argument("--allow-interruption", action="store_true",
                        help="Permit container replacement during an approved maintenance window")
    arguments = parser.parse_args()
    phase = arguments.phase
    if phase == "activate" and (ROOT / "runtime/new-api-bluegreen/state.json").exists():
        parser.error("Managed publishing is active; single-instance activation interrupts live streams. "
                     "Use /opt/us-whitelist/ops/new-api-bluegreen/publish.py")
    if phase == "activate" and not arguments.allow_interruption:
        parser.error("Single-instance activation interrupts live streams; "
                     "use --allow-interruption only during approved maintenance")
    assert os.uname().nodename == "ser021360749320" and os.geteuid() == 0
    assert Path(__file__).resolve().parent == RELEASE and not RELEASE.is_symlink()
    os.umask(0o077)
    with (ROOT / ".production-suite-deploy.lock").open("a") as lock:
        fcntl.flock(lock, fcntl.LOCK_EX | fcntl.LOCK_NB)
        if phase == "prepare":
            prepare()
        elif phase == "activate":
            activate()
        else:
            print(json.dumps(verify_runtime()), flush=True)


if __name__ == "__main__":
    try:
        main()
    except Exception as error:
        if BACKUP.exists():
            save(BACKUP / "phase-error.json", {"status": "failed", "message": str(error)})
        raise
