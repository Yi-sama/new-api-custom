import ast
import hashlib
import json
from pathlib import Path
import subprocess
import sys
import tempfile

from nginx_validation import validate_nginx


RELEASE = Path(__file__).resolve().parent
INGRESS = Path("/etc/nginx/conf.d/api-yisama.conf")
PATHS = [Path("/var/lib/nginx"), Path("/var/log/nginx")]
PATHS += sorted(Path("/var/lib/nginx").iterdir())


def snapshot():
    result = {}
    for path in PATHS:
        stat = path.stat()
        result[str(path)] = (stat.st_uid, stat.st_gid, stat.st_mode, stat.st_ino)
    result["ingress_sha256"] = hashlib.sha256(INGRESS.read_bytes()).hexdigest()
    result["new-api"] = subprocess.run(
        ["docker", "inspect", "new-api", "--format",
         "{{.Id}} {{.State.StartedAt}} {{.RestartCount}} {{.State.Health.Status}}"],
        check=True, capture_output=True, timeout=15,
    ).stdout.decode().strip()
    return result


def check():
    before = snapshot()
    callers = ["deploy.py", "deploy-python310.py", "https-fix/deploy-v2.py",
               "repair-ingress.py", "validate-config.py"]
    for name in callers:
        tree = ast.parse((RELEASE / name).read_text(encoding="utf-8"))
        imports = [node for node in ast.walk(tree) if isinstance(node, ast.ImportFrom)]
        assert any(node.module == "nginx_validation" for node in imports), name
        assert not any(
            isinstance(node, ast.Call) and isinstance(node.func, ast.Attribute)
            and node.func.attr == "memfd_create" for node in ast.walk(tree)
        ), f"Unsafe standalone validation remains: {name}"
        calls = [
            node for node in ast.walk(tree) if isinstance(node, ast.Call)
            and isinstance(node.func, ast.Name) and node.func.id == "validate_nginx"
        ]
        assert len(calls) == 1, f"Candidate check is not routed through helper: {name}"
        context = {
            "validate_nginx": validate_nginx, "RELEASE": RELEASE,
            "target": RELEASE / "api-yisama.proxy-assets.conf",
            "nginx": "server {\nlisten 127.0.0.1:32831;\n"
                     + (RELEASE / "tavern-proxy-assets.nginx.conf").read_text() + "\n}",
        }
        # Execute only the actual candidate-test call, never preparation/activation.
        expression = ast.Expression(body=calls[0])
        exec(compile(ast.fix_missing_locations(expression), name, "eval"), context)
        assert snapshot() == before, f"Production state changed by {name}"
    for _ in range(3):
        validate_nginx(f"include {INGRESS};")
        assert snapshot() == before, "Repeated ingress validation changed production"
    validate_nginx(None)
    assert snapshot() == before, "Full live-config test changed production"
    with tempfile.TemporaryDirectory(prefix="nginx-full-regression-", dir="/run") as directory:
        candidate = Path(directory) / "candidate.conf"
        candidate.write_text(INGRESS.read_text() + "\nthis_is_invalid;\n")
        rejected = False
        try:
            validate_nginx(None, replacements=[(candidate, INGRESS)])
        except RuntimeError as error:
            rejected = "unknown directive" in str(error)
        assert rejected, "Full candidate validation did not reject invalid syntax"
        assert snapshot() == before, "Rejected full candidate changed production"
    rejected = False
    try:
        validate_nginx("this_is_not_a_valid_nginx_directive;")
    except RuntimeError as error:
        rejected = "unknown directive" in str(error)
    assert rejected, "Invalid candidate was not rejected"
    assert snapshot() == before, "Rejected candidate changed production"
    guards = ["deploy.py", "deploy-python310.py", "https-fix/deploy-v2.py"]
    for name in guards:
        result = subprocess.run(
            [sys.executable, str(RELEASE / name), "activate"],
            capture_output=True, timeout=15,
        )
        assert result.returncode == 2 and b"interrupts live streams" in result.stderr, name
        assert snapshot() == before, f"Blocked activation changed production: {name}"
        if Path("/opt/us-whitelist/runtime/new-api-bluegreen/state.json").exists():
            result = subprocess.run(
                [sys.executable, str(RELEASE / name), "activate", "--allow-interruption"],
                capture_output=True, timeout=15,
            )
            assert result.returncode == 2 and b"Managed publishing" in result.stderr, name
            assert snapshot() == before, f"Legacy override bypassed managed publishing: {name}"
    rollback = "/opt/us-whitelist/backups/production-suite-20261003-001/rollback.sh"
    result = subprocess.run(["sh", rollback], capture_output=True, timeout=15)
    assert result.returncode == 2 and b"interrupts live streams" in result.stderr
    assert snapshot() == before, "Blocked rollback changed production"
    if Path("/opt/us-whitelist/runtime/new-api-bluegreen/state.json").exists():
        result = subprocess.run(["sh", rollback, "--allow-interruption"],
                                capture_output=True, timeout=15)
        assert result.returncode == 2 and b"Managed publishing" in result.stderr
        assert snapshot() == before, "Legacy rollback override bypassed managed publishing"
    return {
        "status": "pass", "candidate_callers_checked": callers,
        "repeated_active_config_checks": 3, "invalid_config_rejected": True,
        "activation_guards_checked": guards, "rollback_guard_checked": True,
        "full_config_checked": True, "full_invalid_candidate_rejected": True,
        "production_paths_unchanged": True, "ingress_unchanged": True,
        "new_api_unchanged": before["new-api"], "paid_calls": 0,
    }


if __name__ == "__main__":
    print(json.dumps(check(), indent=2))
