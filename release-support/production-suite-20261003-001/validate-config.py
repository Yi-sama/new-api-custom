import copy
import json
import os
import subprocess
import sys

from nginx_validation import validate_nginx


def run(command, payload=None, **kwargs):
    result = subprocess.run(command, input=payload, capture_output=True, timeout=45, **kwargs)
    if result.returncode:
        raise RuntimeError(f"{command[0]} configuration validation failed: {result.stderr.decode()}")
    return result.stdout


if os.uname().nodename != "ser021360749320":
    raise RuntimeError("US host identity mismatch")
payload = json.load(sys.stdin)
base = [
    "docker", "compose", "-p", "us-whitelist",
    "-f", "/opt/us-whitelist/docker-compose.yml",
    "-f", "/opt/us-whitelist/releases/studio-credit-20261003-2120/backend.override.json",
]
before = json.loads(run(base + ["config", "--format", "json"]))
after = json.loads(run(
    base + ["-f", "-", "config", "--format", "json"],
    json.dumps(payload["backend"]).encode(),
))
original = copy.deepcopy(before)
current = copy.deepcopy(after)
allowed_environment = set(payload["backend"]["services"]["new-api"]["environment"])
allowed_mounts = {mount["target"] for mount in payload["backend"]["services"]["new-api"]["volumes"]}
for config in (original, current):
    service = config["services"]["new-api"]
    service["environment"] = {
        key: value for key, value in service["environment"].items()
        if key not in allowed_environment
    }
    service["volumes"] = [
        mount for mount in service["volumes"] if mount["target"] not in allowed_mounts
    ]
if original != current:
    raise RuntimeError("Unexpected Compose differences outside the approved service delta")
service = after["services"]["new-api"]
for key, value in payload["backend"]["services"]["new-api"]["environment"].items():
    if service["environment"].get(key) != value:
        raise RuntimeError(f"Resolved environment mismatch for {key}")
mounts = {mount["target"]: mount for mount in service["volumes"]}
for expected in payload["backend"]["services"]["new-api"]["volumes"]:
    actual = mounts.get(expected["target"], {})
    if any(actual.get(key) != expected[key] for key in ("source", "target", "type", "read_only")):
        raise RuntimeError("Resolved release mount mismatch")
print("BACKEND_COMPOSE_SCOPE_VERIFIED", flush=True)

tavern = json.loads(run(
    ["docker", "compose", "-p", "us-whitelist-tavern-production", "-f", "-", "config", "--format", "json"],
    json.dumps(payload["tavern"]).encode(),
))
runtime = tavern["services"]["risuai"]
if runtime["network_mode"] != "host" or runtime["environment"]["PORT"] != "32831":
    raise RuntimeError("Unexpected Tavern listener configuration")
if runtime["environment"]["NEW_API_URL"] != "http://127.0.0.1:32794":
    raise RuntimeError("Tavern backend is not the existing production instance")
if runtime["image"] != "sha256:ebfe2f90462722a7a4de65e91990e97fe0d401c70e0e762c5b53302f905ec1c1":
    raise RuntimeError("Unexpected Tavern image")
print("TAVERN_COMPOSE_VERIFIED", flush=True)

nginx = "server {\nlisten 127.0.0.1:32831;\n"
nginx += payload["nginx"] + "\n}\n"
validate_nginx(nginx)
print("NGINX_SNIPPET_SYNTAX_VERIFIED", flush=True)
run(["sh", "-n"], payload["rollback"].encode())
print("ROLLBACK_SYNTAX_VERIFIED", flush=True)
print("NO_ACTIVATION_PERFORMED", flush=True)
