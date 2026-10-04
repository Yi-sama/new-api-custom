import hashlib
import json
import os
from pathlib import Path
import shutil
import subprocess
import time

from nginx_validation import validate_nginx

release = Path("/opt/us-whitelist/releases/production-suite-20261003-001")
backup = Path("/opt/us-whitelist/backups/production-suite-20261003-001")
ingress = Path("/etc/nginx/conf.d/api-yisama.conf")


def run(args, **kwargs):
    return subprocess.run(args, check=True, capture_output=True, timeout=30, **kwargs).stdout


assert os.uname().nodename == "ser021360749320" and os.geteuid() == 0
assert ingress.is_symlink() and ingress.resolve() == release / "api-yisama.conf"
original = ingress.read_text()
old_include = f"include {release}/tavern-locations.nginx.conf;"
assert original.count(old_include) == 1
before = backup / "api-yisama.before-static-fix.conf"
assert not before.exists()
shutil.copy2(ingress, before)
target = release / "api-yisama.proxy-assets.conf"
assert not target.exists()
target.write_text(original.replace(old_include, f"include {release}/tavern-proxy-assets.nginx.conf;"))
validate_nginx(f"include {target};")
temporary = ingress.with_name("api-yisama.conf.proxy-assets-next")
assert not temporary.exists() and not temporary.is_symlink()
try:
    temporary.symlink_to(target)
    os.replace(temporary, ingress)
    run(["nginx", "-t"])
    run(["systemctl", "reload", "nginx"])
    asset = "index-ZrSVS3d8.js"
    expected = hashlib.sha256((release / "risuai-managed/dist/assets" / asset).read_bytes()).hexdigest()
    for attempt in range(20):
        result = subprocess.run(["curl", "-sS", "--max-time", "15", "-w", "\n%{http_code}",
                                 "https://api.yisama.asia/tavern-app/assets/" + asset],
                                capture_output=True, timeout=20, check=True).stdout
        body, status = result.rsplit(b"\n", 1)
        if status == b"200" and hashlib.sha256(body).hexdigest() == expected:
            break
        time.sleep(1)
    else:
        raise RuntimeError("Public static serving not ready")
except Exception:
    temporary.symlink_to(release / "api-yisama.conf")
    os.replace(temporary, ingress)
    run(["nginx", "-t"])
    run(["systemctl", "reload", "nginx"])
    raise
state = {"status": "verified", "public_asset_sha256": expected,
         "config": str(target), "before": str(before), "root_only_permissions_retained": True}
(backup / "ingress-static-fix.json").write_text(json.dumps(state, indent=2) + "\n")
print(json.dumps(state))
