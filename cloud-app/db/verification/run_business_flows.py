"""Run SQL integration scenarios against an explicitly selected test database.

Uses stdlib plus the PostgreSQL psql executable. The application SQL remains the
single source: this runner prepares current db/queries statements dynamically.
"""
import argparse
import os
from pathlib import Path
import re
import subprocess
from urllib.parse import unquote, urlsplit


def main():
    root = Path(__file__).resolve().parents[3]
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--database", required=True, help="Isolated database after migration 010")
    parser.add_argument("--env", type=Path, default=root / "cloud-app/.env")
    parser.add_argument("--psql", default="C:/Program Files/PostgreSQL/18/bin/psql.exe")
    args = parser.parse_args()
    line = next(x for x in args.env.read_text(encoding="utf-8-sig").splitlines() if x.startswith("DATABASE_URL="))
    url = urlsplit(line.split("=", 1)[1].strip().strip("\"'"))
    if args.database == url.path.lstrip("/"):
        parser.error("Select a separate test database, not the configured application database")
    env = os.environ.copy()
    env["PGPASSWORD"] = unquote(url.password or "")
    prepared, names = [], set()
    for file in sorted((root / "cloud-app/db/queries").glob("*.sql")):
        for name, body in re.findall(r"-- name: (\w+)\s*\n(.*?)(?=\n-- name:|\Z)", file.read_text(encoding="utf-8"), re.S):
            if name.lower() in names:
                raise ValueError("Duplicate query name: " + name)
            names.add(name.lower())
            if "REFRESH MATERIALIZED VIEW" not in body:
                prepared.append("PREPARE " + name.lower() + " AS " + body)
    scenario = Path(__file__).with_name("010_business_flows.sql").read_text(encoding="utf-8")
    scenario = scenario.replace("SET LOCAL ROLE kicau_cloud_app;", "SET LOCAL ROLE kicau_cloud_app;\n" + "\n".join(prepared), 1)
    result = subprocess.run(
        [args.psql, "-X", "-w", "-h", url.hostname, "-p", str(url.port or 5432),
         "-U", unquote(url.username or "postgres"), "-d", args.database, "-v", "ON_ERROR_STOP=1", "-f", "-"],
        input=scenario, capture_output=True, text=True, env=env,
    )
    if result.returncode:
        raise SystemExit("SQL verification failed:\n" + result.stderr[-3000:])
    print("PASS:", len(names), "named statements loaded; PostgreSQL scenario rolled back successfully.")


if __name__ == "__main__":
    main()
