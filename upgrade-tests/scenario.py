#!/usr/bin/env python3
import hashlib
import json
import os
import pathlib
import subprocess
import sys
import urllib.request

BASE_URL = "http://127.0.0.1:18085"
EMAIL = "upgradeproof@example.invalid"
PASSWORD = "UpgradeProof-Sentinel-2026!"
NAME = "UpgradeProof Sentinel"
FOLDER = "Upgrade Evidence"
TAG = "Persistent"
TAG_COLOR = "#2563eb"
TITLE = "UpgradeProof retained recording"


def request(path, method="GET", body=None, token=None):
    data = None if body is None else json.dumps(body).encode()
    headers = {"Content-Type": "application/json"}
    if token:
        headers["Authorization"] = f"Bearer {token}"
    req = urllib.request.Request(BASE_URL + path, data=data, headers=headers, method=method)
    with urllib.request.urlopen(req, timeout=30) as response:
        raw = response.read()
        return None if not raw else json.loads(raw)


def compose(*args):
    command = ["docker", "compose", "-f", os.environ["UPGRADEPROOF_COMPOSE_FILE"],
               "-p", os.environ["UPGRADEPROOF_PROJECT"], *args]
    return subprocess.check_output(command, text=True).strip()


def container_evidence(service):
    container_id = compose("ps", "-q", service)
    if not container_id:
        raise AssertionError(f"{service} container is missing")
    inspected = json.loads(subprocess.check_output(["docker", "inspect", container_id], text=True))[0]
    return {
        "container_id": inspected["Id"],
        "image_ref": inspected["Config"]["Image"],
        "image_id": inspected["Image"],
        "mounts": sorted(
            ({"type": m["Type"], "name": m.get("Name"), "destination": m["Destination"]}
             for m in inspected["Mounts"]),
            key=lambda mount: mount["destination"],
        ),
    }


def login():
    return request("/api/auth/login", "POST", {"email": EMAIL, "password": PASSWORD})["accessToken"]


def migration_state():
    output = compose("exec", "-T", "postgres", "psql", "-U", "sendrec", "-d", "sendrec",
                     "-Atc", "SELECT version || ':' || dirty FROM schema_migrations")
    return output


def garage_credentials():
    raw = compose("exec", "-T", "sendrec", "cat", "/run/garage-keys/env")
    values = dict(line.split("=", 1) for line in raw.splitlines())
    return values["S3_ACCESS_KEY"], values["S3_SECRET_KEY"]


def write_json(name, value):
    report_dir = pathlib.Path(os.environ["UPGRADEPROOF_REPORT_DIR"])
    report_dir.mkdir(parents=True, exist_ok=True)
    (report_dir / name).write_text(json.dumps(value, indent=2) + "\n", encoding="utf-8")


def seed():
    request("/api/auth/register", "POST", {"email": EMAIL, "password": PASSWORD, "name": NAME})
    token = login()
    request("/api/user/", "PATCH", {"retentionDays": 90}, token)
    folder = request("/api/folders/", "POST", {"name": FOLDER}, token)
    tag = request("/api/tags/", "POST", {"name": TAG, "color": TAG_COLOR}, token)

    fixture = pathlib.Path("web/e2e/fixtures/test-video.webm").read_bytes()
    video = request("/api/videos/upload", "POST", {
        "title": TITLE, "fileSize": len(fixture), "contentType": "video/webm"
    }, token)
    upload = urllib.request.Request(video["uploadUrl"], data=fixture,
                                    headers={"Content-Type": "video/webm"}, method="PUT")
    with urllib.request.urlopen(upload, timeout=30) as response:
        if response.status not in (200, 201):
            raise AssertionError(f"object upload returned {response.status}")

    request(f"/api/videos/{video['id']}/folder", "PUT", {"folderId": folder["id"]}, token)
    request(f"/api/videos/{video['id']}/tags", "PUT", {"tagIds": [tag["id"]]}, token)

    sentinel = {
        "user": {"email": EMAIL, "name": NAME, "retentionDays": 90},
        "folder": {"id": folder["id"], "name": FOLDER},
        "tag": {"id": tag["id"], "name": TAG, "color": TAG_COLOR},
        "video": {"id": video["id"], "title": TITLE, "status": "uploading",
                  "shareToken": video["shareToken"]},
        "object": {"url": video["uploadUrl"].split("?", 1)[0], "size": len(fixture),
                   "sha256": hashlib.sha256(fixture).hexdigest()},
    }
    write_json("sentinel.json", sentinel)
    write_json("source-identities.json", {
        "sendrec": container_evidence("sendrec"),
        "postgres": container_evidence("postgres"),
        "garage": container_evidence("garage"),
        "migration": migration_state(),
    })
    print("Seeded a user, retained setting, folder, tag, video metadata, and 1,459-byte Garage object.")


def verify():
    report_dir = pathlib.Path(os.environ["UPGRADEPROOF_REPORT_DIR"])
    sentinel = json.loads((report_dir / "sentinel.json").read_text(encoding="utf-8"))
    source = json.loads((report_dir / "source-identities.json").read_text(encoding="utf-8"))
    token = login()

    user = request("/api/user/", token=token)
    assert {key: user[key] for key in sentinel["user"]} == sentinel["user"]
    folders = request("/api/folders/", token=token)
    assert [(f["id"], f["name"], f["videoCount"]) for f in folders] == [
        (sentinel["folder"]["id"], FOLDER, 1)]
    tags = request("/api/tags/", token=token)
    assert [(t["id"], t["name"], t["color"], t["videoCount"]) for t in tags] == [
        (sentinel["tag"]["id"], TAG, TAG_COLOR, 1)]
    videos = request("/api/videos/", token=token)
    assert len(videos) == 1
    video = videos[0]
    for key, expected in sentinel["video"].items():
        assert video[key] == expected, f"video {key}: expected {expected!r}, got {video[key]!r}"
    assert video["folderId"] == sentinel["folder"]["id"]
    assert video["tags"] == [{"id": sentinel["tag"]["id"], "name": TAG, "color": TAG_COLOR}]

    access_key, secret_key = garage_credentials()
    object_bytes = subprocess.check_output([
        "curl", "--fail", "--silent", "--show-error", "--aws-sigv4",
        "aws:amz:eu-central-1:s3", "--user", f"{access_key}:{secret_key}", sentinel["object"]["url"]
    ])
    assert len(object_bytes) == sentinel["object"]["size"]
    assert hashlib.sha256(object_bytes).hexdigest() == sentinel["object"]["sha256"]

    target = {
        "sendrec": container_evidence("sendrec"),
        "postgres": container_evidence("postgres"),
        "garage": container_evidence("garage"),
        "migration": migration_state(),
    }
    assert source["sendrec"]["container_id"] != target["sendrec"]["container_id"]
    assert source["sendrec"]["image_id"] != target["sendrec"]["image_id"]
    assert source["postgres"]["container_id"] == target["postgres"]["container_id"]
    assert source["postgres"]["mounts"] == target["postgres"]["mounts"]
    assert source["garage"]["container_id"] == target["garage"]["container_id"]
    assert source["garage"]["mounts"] == target["garage"]["mounts"]
    assert source["migration"] == "59:false"
    assert target["migration"] == "59:false"
    write_json("target-identities.json", target)
    print("Exact SendRec domain state, PostgreSQL/Garage identity, object bytes, and migration ledger are preserved.")


if __name__ == "__main__":
    if len(sys.argv) != 2 or sys.argv[1] not in ("seed", "verify"):
        raise SystemExit("usage: scenario.py seed|verify")
    globals()[sys.argv[1]]()
