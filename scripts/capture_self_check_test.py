import importlib.util
import json
from pathlib import Path
import tempfile
import unittest

spec = importlib.util.spec_from_file_location("capture", Path(__file__).with_name("capture-self-check.py"))
capture = importlib.util.module_from_spec(spec)
spec.loader.exec_module(capture)


class EvidenceTests(unittest.TestCase):
    def fixture(self, directory):
        model = {"metadata": {"revision": "revision", "modified": "false",
                              "options": {"startup.GOMAXPROCS": ["2"]}}}
        (directory / "model.json").write_text(json.dumps(model))
        return {"schemaVersion": 1, "runId": "run", "startedAt": "start", "finishedAt": "end",
                "status": "unsupported", "exitCode": 5, "toolRevision": "revision",
                "toolModified": "false", "patterns": ["./cmd/gotla"], "runtimeProcs": 2,
                "sourceDirectory": str(directory), "artifacts": {"model.json": {
                    "path": str(directory / "model.json"), "sha256": capture.digest(directory / "model.json")}}}

    def validate(self, directory, result, code=5):
        (directory / "result.json").write_text(json.dumps(result))
        return capture.validate_result(directory, "revision", 2, code, "jar-hash", directory)

    def test_unsupported_is_not_a_pass(self):
        with tempfile.TemporaryDirectory() as tmp:
            directory = Path(tmp)
            result = self.fixture(directory)
            self.assertEqual(self.validate(directory, result)["status"], "unsupported")
            with self.assertRaises(capture.EvidenceError):
                self.validate(directory, result, 0)

    def test_forged_or_stale_evidence(self):
        for broken in ("revision", "modified", "pattern", "profile", "trust", "hash", "path",
                       "command", "tlc-exit", "stale-tla", "model-profile", "schema", "unfinished",
                       "unexpected-artifact", "model-symlink", "false-pass"):
            with self.subTest(broken=broken), tempfile.TemporaryDirectory() as tmp:
                directory = Path(tmp)
                result = self.fixture(directory)
                if broken == "revision": result["toolRevision"] = "old"
                if broken == "modified": result["toolModified"] = "true"
                if broken == "pattern": result["patterns"] = ["./examples/unbuffered"]
                if broken == "profile": result["runtimeProcs"] = 3
                if broken == "trust": result["trustedCalls"] = ["unknown"]
                if broken == "hash": result["artifacts"]["model.json"]["sha256"] = "wrong"
                if broken == "path": result["artifacts"]["model.json"]["path"] = str(directory / "other")
                if broken == "command": result["command"] = ["java"]
                if broken == "tlc-exit": result["tlcExitCode"] = 0
                if broken == "stale-tla": (directory / "model.tla").write_text("old")
                if broken == "model-profile":
                    model = json.loads((directory / "model.json").read_text())
                    model["metadata"]["options"]["startup.GOMAXPROCS"] = ["3"]
                    (directory / "model.json").write_text(json.dumps(model))
                    result["artifacts"]["model.json"]["sha256"] = capture.digest(directory / "model.json")
                if broken == "schema": result["schemaVersion"] = 9
                if broken == "unfinished": del result["finishedAt"]
                if broken == "unexpected-artifact": result["artifacts"]["../other"] = {}
                if broken == "model-symlink":
                    (directory / "model.json").rename(directory / "other")
                    (directory / "model.json").symlink_to(directory / "other")
                if broken == "false-pass":
                    result.update(status="passed", exitCode=0)
                with self.assertRaises(capture.EvidenceError):
                    self.validate(directory, result, 0 if broken == "false-pass" else 5)

    def test_completed_checker_requires_all_artifacts_and_pin(self):
        with tempfile.TemporaryDirectory() as tmp:
            directory = Path(tmp)
            result = self.fixture(directory)
            result.update(status="deadlock", exitCode=3, jarSHA256="jar-hash",
                          command=["java"], tlcVersion="1.8.0", tlcExitCode=11)
            for name in ("model.tla", "model.cfg", "tlc.log"):
                path = directory / name
                path.write_text("fixture")
                result["artifacts"][name] = {"path": str(path), "sha256": capture.digest(path)}
            self.assertEqual(self.validate(directory, result, 3)["status"], "deadlock")
            result["jarSHA256"] = "unapproved"
            with self.assertRaises(capture.EvidenceError): self.validate(directory, result, 3)

    def test_snapshot_detects_drift(self):
        with tempfile.TemporaryDirectory() as tmp:
            path = Path(tmp) / "source.go"
            path.write_text("before")
            before = capture.snapshot([path, path])
            self.assertEqual(len(before), 1)
            path.write_text("after")
            self.assertNotEqual(before, capture.snapshot([path]))

    def test_package_json_stream(self):
        self.assertEqual(list(capture.json_objects(' {"a": 1}\n {"b": 2}\n')),
                         [{"a": 1}, {"b": 2}])
        with self.assertRaises(ValueError): list(capture.json_objects('{}\nnot json'))


if __name__ == "__main__":
    unittest.main()
