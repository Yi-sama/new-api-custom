import copy
import hashlib
import json
import pathlib
import platform
import shlex
import subprocess
import sys
import time
import urllib.parse
import urllib.request

ROOT = pathlib.Path(__file__).resolve().parent
NGINX = pathlib.Path("/etc/nginx/conf.d/api-yisama.conf")
NAMES = ["new-api-accrual", "new-api"]
OLD = {
    "new-api": "6d998d706abaf43c79530f08c839a454a8b88b3389f34358ee48e61db96a368f",
    "new-api-accrual": "2625005af6ce83b5fb127150195a9393ee73183221fe0607b0d3b9b0156f120d",
}
assert platform.node() == "ser021360749320"


def run(args, timeout=180):
    return subprocess.check_output(args, text=True, timeout=timeout)


def read(path):
    return json.loads(path.read_text(encoding="utf-8"))


def save(path, value):
    path.write_text(json.dumps(value, indent=2) + "\n", encoding="utf-8")
    path.chmod(0o600)


def sha(path):
    digest = hashlib.sha256()
    with open(path, "rb") as stream:
        for block in iter(lambda: stream.read(1024 * 1024), b""):
            digest.update(block)
    return digest.hexdigest()


def inspect(name):
    return json.loads(run(["docker", "inspect", name]))[0]


def hashes(name):
    return [line.split()[0] for line in run(
        ["docker", "exec", name, "sha256sum", "/new-api", "/proc/1/exe"]
    ).splitlines()]


def resources():
    memory = dict(line.split(":", 1) for line in pathlib.Path("/proc/meminfo").read_text().splitlines())
    available = int(memory["MemAvailable"].split()[0]) // 1024
    assert available >= 2048, "Insufficient available memory"
    return {"available_mib": available}


def database_args():
    env = dict(v.split("=", 1) for v in inspect("new-api")["Config"]["Env"])
    dsn = urllib.parse.urlsplit(env["SQL_DSN"])
    return ["docker", "exec", "-i", "sub2api-postgres", "psql",
            "-U", urllib.parse.unquote(dsn.username or ""), "-d", dsn.path.lstrip("/"),
            "-v", "ON_ERROR_STOP=1"]


def check_wallet_schema():
    counts = run(database_args() + ["-Atc",
        "SELECT count(*) FROM information_schema.columns WHERE table_schema='public' "
        "AND table_name='wallet_pre_consume_records'; "
        "SELECT count(*) FROM pg_indexes WHERE schemaname='public' "
        "AND tablename='wallet_pre_consume_records'"]).strip().splitlines()
    assert counts == ["8", "4"], "Wallet migration must complete before switching services"


def healthy(name):
    for _ in range(60):
        current = inspect(name)
        assert current["State"]["Running"], "Service is not running"
        health = current["State"].get("Health", {}).get("Status")
        if health == "healthy":
            port = next(iter(current["NetworkSettings"]["Ports"].values()))[0]["HostPort"]
            with urllib.request.urlopen(f"http://127.0.0.1:{port}/api/status", timeout=15) as response:
                assert response.status == 200 and json.load(response)["success"] is True
            return current
        assert health != "unhealthy", "Service is unhealthy"
        time.sleep(2)
    raise RuntimeError("Health check timed out")


def graceful_stop(name):
    subprocess.run(["docker", "stop", "--timeout", "150", name], check=True, timeout=170)
    current = inspect(name)
    logs = run(["docker", "logs", "--tail", "25", name])
    assert current["State"]["ExitCode"] == 0 and not current["State"]["OOMKilled"], "Shutdown failed"
    assert "server exited" in logs and "server forced to shutdown" not in logs, "Requests did not drain"


def check_container(name, before, expected_sha):
    after = healthy(name)
    assert hashes(name) == [expected_sha, expected_sha]
    assert after["Image"] == before["Image"]
    assert sorted(after["Config"]["Env"]) == sorted(before["Config"]["Env"])
    for key in ["Cmd", "Entrypoint", "User", "WorkingDir", "Healthcheck"]:
        assert after["Config"][key] == before["Config"][key], key
    for key in ["PortBindings", "RestartPolicy", "Memory", "MemoryReservation", "NanoCpus",
                "CpuQuota", "CpuPeriod", "NetworkMode"]:
        assert after["HostConfig"][key] == before["HostConfig"][key], key
    expected = sorted(
        (str(ROOT / "new-api") if m["Destination"] == "/new-api" else m["Source"],
         m["Destination"], m["RW"], m["Type"]) for m in before["Mounts"]
    )
    actual = sorted((m["Source"], m["Destination"], m["RW"], m["Type"]) for m in after["Mounts"])
    assert expected == actual
    return after


action = sys.argv[1]
manifest = read(ROOT / "build-manifest.json")
new_sha = manifest["binarySha256"]

if action == "prepare":
    resources()
    assert sha(ROOT / "new-api") == new_sha
    backup = ROOT / "backup"
    backup.mkdir(mode=0o700)
    nginx_backup = NGINX.with_name(NGINX.name + ".settled-balance-20261103-r2.bak")
    assert not nginx_backup.exists(), "Nginx backup already exists"
    nginx_backup.write_bytes(NGINX.read_bytes())
    nginx_backup.chmod(0o600)
    state = {"nginx_sha256": sha(NGINX), "nginx_backup": str(nginx_backup), "services": {}}
    for name in NAMES:
        current = healthy(name)
        assert hashes(name) == [OLD[name], OLD[name]]
        labels = current["Config"]["Labels"]
        command = ["docker", "compose", "--project-directory", labels["com.docker.compose.project.working_dir"],
                   "-p", labels["com.docker.compose.project"]]
        files = labels["com.docker.compose.project.config_files"].split(",")
        for file in files:
            command += ["-f", file]
        resolved = json.loads(run(command + ["config", "--format", "json"]))
        save(backup / f"{name}.container.json", current)
        save(backup / f"{name}.compose.json", resolved)
        expected = copy.deepcopy(resolved)
        mount = next(m for m in expected["services"][name]["volumes"] if m["target"] == "/new-api")
        old_binary = mount["source"]
        assert sha(old_binary) == OLD[name]
        mount["source"] = str(ROOT / "new-api")
        override = ROOT / f"{name}.override.json"
        save(override, {"services": {name: {"volumes": [mount]}}})
        deploy_base = command + ["-f", str(override)]
        planned = json.loads(run(deploy_base + ["config", "--format", "json"]))
        assert planned == expected, "Only the executable mount may change"
        flags = ["up", "-d", "--no-deps", "--no-build", "--pull", "never", name]
        save(backup / f"{name}.planned.json", planned)
        state["services"][name] = {
            "id": current["Id"], "old_binary": old_binary,
            "original_files": {file: sha(file) for file in files},
            "deploy_base": deploy_base, "deploy": deploy_base + flags,
            "rollback": command[:command.index("-f")] + ["-f", str(backup / f"{name}.compose.json")] + flags,
        }
    state["unchanged"] = {c["Name"].lstrip("/"): c["Id"] for c in json.loads(run(
        ["docker", "inspect"] + run(["docker", "ps", "-q"]).split()
    )) if c["Name"].lstrip("/") not in NAMES}
    env = dict(v.split("=", 1) for v in inspect("new-api")["Config"]["Env"])
    dsn = urllib.parse.urlsplit(env["SQL_DSN"])
    database_env = dict(v.split("=", 1) for v in inspect("sub2api-postgres")["Config"]["Env"])
    database_admin = database_env.get("POSTGRES_USER", "postgres")
    dump = backup / "newapi-before.dump"
    with dump.open("xb") as stream:
        dump.chmod(0o600)
        subprocess.run(["docker", "exec", "sub2api-postgres", "pg_dump", "-U", database_admin,
                        "-d", dsn.path.lstrip("/"), "-Fc", "--no-owner", "--no-acl"],
                       stdout=stream, check=True, timeout=300)
    assert dump.stat().st_size > 1000000
    with dump.open("rb") as stream:
        subprocess.run(["docker", "exec", "-i", "sub2api-postgres", "pg_restore", "--list"],
                       stdin=stream, stdout=subprocess.DEVNULL, check=True, timeout=60)
    state["database_backup"] = {"path": str(dump), "sha256": sha(dump), "size": dump.stat().st_size}
    save(ROOT / "prepared.json", state)
    (ROOT / "new-api").chmod(0o755)
    print(json.dumps({"prepared": True, "database_backup": state["database_backup"],
                      "rollback": {n: shlex.join(s["rollback"]) for n, s in state["services"].items()},
                      **resources()}), flush=True)
elif action == "migrate":
    state = read(ROOT / "prepared.json")
    dump = pathlib.Path(state["database_backup"]["path"])
    assert sha(dump) == state["database_backup"]["sha256"]
    args = database_args()
    assert not run(args + ["-Atc",
        "SELECT to_regclass('public.wallet_pre_consume_records')"]).strip(), "Inspect existing wallet schema first"
    sql = (ROOT / "wallet-migration.sql").read_text(encoding="utf-8")
    assert len(sql.splitlines()) == 4 and sql.startswith('CREATE TABLE "wallet_pre_consume_records"')
    subprocess.run(args + ["--single-transaction"], input=sql, text=True, check=True, timeout=60)
    check_wallet_schema()
    save(ROOT / "migration-completed.json", {
        "migrated": True, "table": "wallet_pre_consume_records",
        "sql_sha256": hashlib.sha256(sql.encode()).hexdigest(), "existing_tables_changed": False,
    })
    print(json.dumps({"migrated": True, "existing_tables_changed": False}), flush=True)
elif action == "deploy":
    state = read(ROOT / "prepared.json")
    resources()
    check_wallet_schema()
    assert sha(ROOT / "new-api") == new_sha and sha(NGINX) == state["nginx_sha256"]
    for name in NAMES:
        s = state["services"][name]
        assert inspect(name)["Id"] == s["id"] and hashes(name) == [OLD[name], OLD[name]]
        assert all(sha(file) == digest for file, digest in s["original_files"].items())
        assert json.loads(run(s["deploy_base"] + ["config", "--format", "json"])) == read(
            ROOT / "backup" / f"{name}.planned.json")
    updated = []
    routed = False
    try:
        # The existing standby serves new requests while old main requests drain.
        name = "new-api-accrual"
        graceful_stop(name)
        subprocess.run(state["services"][name]["deploy"], check=True, timeout=180)
        check_container(name, read(ROOT / "backup" / f"{name}.container.json"), new_sha)
        updated.append(name)
        original = pathlib.Path(state["nginx_backup"]).read_bytes()
        source, target = b"http://127.0.0.1:32794", b"http://127.0.0.1:32840"
        assert original.count(source) >= 1 and original.count(target) == 0
        NGINX.write_bytes(original.replace(source, target))
        subprocess.run(["nginx", "-t"], check=True, timeout=30)
        subprocess.run(["nginx", "-s", "reload"], check=True, timeout=30)
        routed = True
        for _ in range(180):
            connections = run(["ss", "-Htn", "state", "established", "( dport = :32794 )"]).splitlines()
            if not connections:
                break
            time.sleep(1)
        else:
            raise RuntimeError("Main still has in-flight connections; refusing to stop it")
        name = "new-api"
        graceful_stop(name)
        subprocess.run(state["services"][name]["deploy"], check=True, timeout=180)
        check_container(name, read(ROOT / "backup" / f"{name}.container.json"), new_sha)
        updated.append(name)
        NGINX.write_bytes(original)
        subprocess.run(["nginx", "-t"], check=True, timeout=30)
        subprocess.run(["nginx", "-s", "reload"], check=True, timeout=30)
        routed = False
        assert sha(NGINX) == state["nginx_sha256"]
        for name, identity in state["unchanged"].items():
            assert inspect(name)["Id"] == identity, "Unrelated container changed"
        result = {"deployed": True, "source_commit": manifest["sourceCommit"], "sha256": new_sha,
                  "services": {n: healthy(n)["State"]["StartedAt"] for n in NAMES},
                  "nginx_restored": True, **resources()}
        save(ROOT / "deployed.json", result)
        print(json.dumps(result), flush=True)
    except Exception:
        if routed:
            main = inspect("new-api")
            if main["State"]["Running"]:
                NGINX.write_bytes(pathlib.Path(state["nginx_backup"]).read_bytes())
                subprocess.run(["nginx", "-t"], check=True, timeout=30)
                subprocess.run(["nginx", "-s", "reload"], check=True, timeout=30)
        # Leave additive tables and live billing data intact; never restore a live DB dump.
        print(json.dumps({"deployment_failed": True, "updated": updated,
                          "rollback_commands": {n: shlex.join(s["rollback"]) for n, s in state["services"].items()}}),
              flush=True)
        raise
elif action == "rollback":
    state = read(ROOT / "prepared.json")
    for name in reversed(NAMES):
        graceful_stop(name)
        subprocess.run(state["services"][name]["rollback"], check=True, timeout=180)
        healthy(name)
        assert hashes(name) == [OLD[name], OLD[name]]
    NGINX.write_bytes(pathlib.Path(state["nginx_backup"]).read_bytes())
    subprocess.run(["nginx", "-t"], check=True, timeout=30)
    subprocess.run(["nginx", "-s", "reload"], check=True, timeout=30)
    print(json.dumps({"rolled_back": True, "database_preserved": True}), flush=True)
else:
    raise ValueError("Expected prepare, migrate, deploy or rollback")
