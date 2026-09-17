#!/usr/bin/env python3
"""Capture a real-CLI self-check attempt, not a full-bootstrap completion claim."""
import argparse
import hashlib
import json
import os
from pathlib import Path
import re
import signal
import shutil
import stat
import subprocess
import sys
import time


class EvidenceError(Exception):
    pass


def digest(path):
    if not stat.S_ISREG(Path(path).stat().st_mode):
        raise EvidenceError("evidence input is not a regular file")
    h = hashlib.sha256()
    with Path(path).open("rb") as stream:
        for block in iter(lambda: stream.read(1024 * 1024), b""):
            h.update(block)
    return h.hexdigest()


def json_objects(text):
    decoder = json.JSONDecoder()
    offset = 0
    while offset < len(text):
        if text[offset].isspace():
            offset += 1
            continue
        value, offset = decoder.raw_decode(text, offset)
        yield value


def snapshot(paths):
    return {str(p): digest(p) for p in sorted(set(paths))}


def validate_result(directory, revision, procs, exit_code, jar_hash, root):
    result = json.loads((directory / "result.json").read_text())
    if (result.get("schemaVersion") != 1 or not result.get("runId")
            or not result.get("startedAt") or not result.get("finishedAt")):
        raise EvidenceError("result envelope incomplete")
    codes = {"passed": 0, "deadlock": 3, "synchronization-error": 4,
             "unsupported": 5, "incomplete": 6, "analysis-error": 1, "tool-error": 1}
    if codes.get(result.get("status")) != exit_code or result.get("exitCode") != exit_code:
        raise EvidenceError("nonterminal or inconsistent checker result")
    if (result.get("toolRevision") != revision or result.get("toolModified") != "false"
            or result.get("patterns") != ["./cmd/gotla"]
            or Path(result.get("sourceDirectory", "")).resolve() != root
            or result.get("runtimeProcs") != procs or result.get("trustedCalls")):
        raise EvidenceError("self-input/profile/tool provenance mismatch")
    artifacts = result.get("artifacts", {})
    for name, artifact in artifacts.items():
        if name not in {"model.json", "model.tla", "model.cfg", "tlc.log"}:
            raise EvidenceError("unexpected artifact")
        path = directory / name
        if path.is_symlink() or Path(artifact["path"]).resolve() != path.resolve():
            raise EvidenceError("artifact path mismatch")
        if digest(path) != artifact["sha256"]:
            raise EvidenceError("artifact hash mismatch")
    if "model.json" in artifacts:
        model = json.loads((directory / "model.json").read_text())
        metadata = model.get("metadata", {})
        if (metadata.get("revision") != revision or metadata.get("modified") != "false"
                or metadata.get("options", {}).get("startup.GOMAXPROCS") != [str(procs)]):
            raise EvidenceError("model provenance/profile mismatch")
    if result["status"] == "unsupported":
        if (set(artifacts) != {"model.json"} or result.get("command")
                or result.get("tlcExitCode") is not None or result.get("tlcVersion")
                or any((directory / name).exists() for name in ("model.tla", "model.cfg", "tlc.log"))):
            raise EvidenceError("unsupported self-input has executable/checker evidence")
    if result["status"] in {"passed", "deadlock", "synchronization-error"}:
        if (set(artifacts) != {"model.json", "model.tla", "model.cfg", "tlc.log"}
                or result.get("jarSHA256") != jar_hash or not result.get("command")
                or not result.get("tlcVersion") or result.get("tlcExitCode") is None):
            raise EvidenceError("completed checker evidence missing")
    return result


def capture(args):
    root = Path(__file__).resolve().parent.parent
    out = Path(args.out).expanduser().resolve()
    if out == root or root in out.parents or out in root.parents:
        raise EvidenceError("output must be outside the source checkout and its ancestors")
    out.parent.mkdir(parents=True, exist_ok=True)
    out.mkdir()  # Never overwrite previous evidence, including failed attempts.
    manifest = {"schemaVersion": 1, "status": "capturing", "goalComplete": False,
                "scope": "actual CLI self-check attempt; not full bootstrap acceptance",
                "sourceDirectory": str(root), "runtimeProcs": args.runtime_procs,
                "commands": [], "environmentOverrides": {"GOFLAGS": "", "GOWORK": "off",
                    "GOTOOLCHAIN": "local", "GOMAXPROCS": str(args.runtime_procs)},
                "limitations": ["No complete finite argv/I/O/lifecycle profile is established.",
                    "No production mutation experiment or compiler-soundness proof is supplied.",
                    "No concurrent source/cache writer is permitted during capture.",
                    "Selected package files and Go tools are fingerprinted, not all native headers/tools or host state.",
                    "The subprocess deadline is not a whole-tree memory/disk sandbox."]}
    env = {**os.environ, **manifest["environmentOverrides"]}

    def run(argv, log, allow_failure=False):
        record = {"argv": argv, "log": log, "startedAt": time.time()}
        manifest["commands"].append(record)
        with (out / log).open("wb") as stream, (out / (log + ".stderr")).open("wb") as errors:
            process = subprocess.Popen(argv, cwd=root, env=env, stdout=stream,
                                       stderr=errors, stdin=subprocess.DEVNULL,
                                       start_new_session=True)
            try:
                code = process.wait(timeout=args.deadline)
            except BaseException:
                try:
                    os.killpg(process.pid, signal.SIGKILL)
                except ProcessLookupError:
                    pass
                process.wait()
                raise
        record.update(exitCode=code, finishedAt=time.time(), sha256=digest(out / log),
                      stderrLog=log + ".stderr", stderrSHA256=digest(out / (log + ".stderr")))
        if code and not allow_failure:
            raise EvidenceError(f"command failed ({code}); see {log}")
        return code

    try:
        run(["git", "status", "--porcelain", "--untracked-files=all"], "git-before.log")
        if (out / "git-before.log").read_text().strip():
            raise EvidenceError("source checkout must be clean")
        run(["git", "rev-parse", "HEAD"], "revision.log")
        revision = (out / "revision.log").read_text().strip()
        manifest["revision"] = revision
        pins = (root / "scripts/tlc.env").read_text()
        match = re.search(r"^TLC_SHA256=([0-9a-f]{64})$", pins, re.MULTILINE)
        if not match:
            raise EvidenceError("missing pinned TLC checksum")
        jar = Path(args.tlc_jar).expanduser().resolve(strict=True)
        jar_hash = digest(jar)
        if jar_hash != match[1]:
            raise EvidenceError("TLC checksum mismatch")
        manifest["checkerJar"] = {"path": str(jar), "sha256": jar_hash}
        run(["go", "env", "-json", "GOOS", "GOARCH", "GOAMD64", "CGO_ENABLED",
             "GOEXPERIMENT", "GOROOT", "GOTOOLDIR", "GOVERSION", "GOMOD", "GOWORK"], "go-env.json")
        goenv = json.loads((out / "go-env.json").read_text())
        if (goenv["GOOS"], goenv["GOARCH"]) != ("linux", "amd64"):
            raise EvidenceError("runtime profile requires linux/amd64")
        run(["git", "ls-files", "-z"], "tracked-files.bin")
        tracked = [root / name for name in (out / "tracked-files.bin").read_text().split("\0") if name]
        run(["go", "list", "-deps", "-json", "./cmd/gotla"], "packages.json")
        paths = list(tracked)
        fields = ("GoFiles", "CgoFiles", "CFiles", "CXXFiles", "MFiles", "HFiles",
                  "FFiles", "SFiles", "SwigFiles", "SwigCXXFiles", "SysoFiles", "EmbedFiles")
        for package in json_objects((out / "packages.json").read_text()):
            if package.get("Error") or package.get("DepsErrors"):
                raise EvidenceError("package loading error")
            module = package.get("Module", {})
            for item in (module, module.get("Replace", {})):
                if item.get("GoMod"):
                    paths.append(Path(item["GoMod"]))
            for field in fields:
                paths.extend(Path(package["Dir"]) / name for name in package.get(field, []))
        tool_dir = Path(goenv["GOTOOLDIR"])
        paths.extend(tool_dir / name for name in ("compile", "asm", "link"))
        paths.extend([Path(shutil.which("go")), Path(goenv["GOROOT"]) / "bin/go", jar])
        before = snapshot(paths)
        manifest["inputs"] = before
        binary = out / "gotla"
        run(["go", "build", "-o", str(binary), "./cmd/gotla"], "build.log")
        manifest["binarySHA256"] = digest(binary)
        analysis = out / "analysis"
        code = run([str(binary), "check", "-runtime-procs", str(args.runtime_procs),
                    "-out", str(analysis), "-tlc-jar", str(jar), "./cmd/gotla"],
                   "self-check.log", allow_failure=True)
        run(["git", "status", "--porcelain", "--untracked-files=all"], "git-after.log")
        run(["git", "rev-parse", "HEAD"], "revision-after.log")
        if ((out / "git-after.log").read_text().strip()
                or (out / "revision-after.log").read_text().strip() != revision
                or snapshot(paths) != before or digest(binary) != manifest["binarySHA256"]):
            raise EvidenceError("source/tool/input drift during self-check")
        result = validate_result(analysis, revision, args.runtime_procs, code, jar_hash, root)
        manifest.update(status="captured", checkerStatus=result["status"], exitCode=code,
                        resultSHA256=digest(analysis / "result.json"))
        return code
    except BaseException as error:
        manifest.update(status="capture-failed", error=f"{type(error).__name__}: {error}")
        raise
    finally:
        (out / "manifest.json").write_text(json.dumps(manifest, indent=2, sort_keys=True) + "\n")
        print(f"Evidence: {out / 'manifest.json'}", file=sys.stderr)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--out", required=True, help="new evidence directory outside checkout")
    parser.add_argument("--tlc-jar", required=True)
    parser.add_argument("--runtime-procs", type=int, default=2)
    parser.add_argument("--deadline", type=int, default=1800, help="per-command seconds (1..86400)")
    args = parser.parse_args()
    if not 1 <= args.runtime_procs <= 1024 or not 1 <= args.deadline <= 86400:
        parser.error("runtime-procs must be 1..1024 and deadline 1..86400")
    try:
        return capture(args)
    except (EvidenceError, OSError, ValueError, subprocess.TimeoutExpired) as error:
        print(f"Capture failed: {error}", file=sys.stderr)
        return 1


if __name__ == "__main__":
    sys.exit(main())
