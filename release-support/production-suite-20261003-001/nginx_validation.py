import os
from pathlib import Path
import pwd
import subprocess
import tempfile


def validate_nginx(http_config, replacements=(), dump=False):
    """Check US candidate configs without touching live Nginx scratch paths."""
    if os.geteuid() != 0 or os.uname().nodename != "ser021360749320":
        raise RuntimeError("US root identity required for isolated Nginx validation")
    worker_uid = pwd.getpwnam("www-data").pw_uid
    with tempfile.TemporaryDirectory(prefix="nginx-validation-", dir="/run") as directory:
        root = Path(directory)
        for name in ("lib", "log", "cache"):
            (root / name).mkdir()
        config = (
            "user www-data;\n"
            "error_log /dev/stderr;\n"
            f"pid {root}/nginx.pid;\n"
            "events {}\nhttp {\n"
            "access_log off;\n"
            "client_body_temp_path /var/lib/nginx/body;\n"
            "proxy_temp_path /var/lib/nginx/proxy;\n"
            "fastcgi_temp_path /var/lib/nginx/fastcgi;\n"
            "uwsgi_temp_path /var/lib/nginx/uwsgi;\n"
            "scgi_temp_path /var/lib/nginx/scgi;\n"
            + (http_config or "") + "\n}\n"
        )
        (root / "nginx.conf").write_text(config, encoding="utf-8")
        config_path = "/etc/nginx/nginx.conf" if http_config is None else str(root / "nginx.conf")
        mounts = []
        for source, target in replacements:
            mounts.extend((str(Path(source).resolve(strict=True)),
                           str(Path(target).resolve(strict=True))))
        # Includes retain their real paths; only test-process scratch mounts change.
        command = [
            "unshare", "--mount", "--propagation", "private", "--",
            "sh", "-eu", "-c",
            'mount --bind "$1" /var/lib/nginx\n'
            'mount --bind "$2" /var/log/nginx\n'
            'if [ -d /var/cache/nginx ]; then mount --bind "$3" /var/cache/nginx; fi\n'
            'prefix="$4"; config="$5"; testflag="$6"; shift 6\n'
            'while [ "$#" -gt 0 ]; do mount --bind "$1" "$2"; shift 2; done\n'
            'if [ "$config" = /etc/nginx/nginx.conf ]; then\n'
            '  exec nginx "$testflag" -q -c "$config"\n'
            'fi\n'
            'exec nginx "$testflag" -q -p "$prefix/" -c "$config"',
            "nginx-validation", str(root / "lib"), str(root / "log"),
            str(root / "cache"), str(root), config_path, "-T" if dump else "-t", *mounts,
        ]
        result = subprocess.run(command, capture_output=True, timeout=45)
        if result.returncode:
            raise RuntimeError(
                "Isolated Nginx validation failed: "
                + result.stderr.decode("utf-8", errors="replace")
            )
        for path in (root / "lib").iterdir():
            if path.is_dir() and path.stat().st_uid != worker_uid:
                raise RuntimeError(f"Nginx test worker ownership mismatch: {path.name}")
        return result.stdout
