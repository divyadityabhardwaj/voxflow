import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import test from "node:test";
import ts from "typescript";

// Exercise the shared TypeScript logic without a browser or another test dependency.
const source = await readFile(new URL("../src/lib/readiness.ts", import.meta.url), "utf8");
const { outputText } = ts.transpileModule(source, { compilerOptions: { module: ts.ModuleKind.ESNext } });
const { evaluateReadiness } = await import(`data:text/javascript;base64,${Buffer.from(outputText).toString("base64")}`);
const ready = {
  microphone: "authorized", accessibility: true, transcription: "groq", groqKeySet: true,
  engineReady: true, modelDownloaded: true, modelReady: true, refinementMode: "refine",
  cleanupProvider: { name: "Gemini", needsKey: true, keySet: true },
};

test("configured cloud dictation ignores local model and engine availability", () => {
  const result = evaluateReadiness({ ...ready, engineReady: false, modelDownloaded: false, modelReady: false });
  assert.equal(result.canRecord, true);
  assert.equal(result.speech, "Groq (cloud)");
  assert.equal(result.cleanup, "Gemini");
  assert.equal(result.issue, null);
});

test("a missing Groq key blocks dictation before recording starts", () => {
  const result = evaluateReadiness({ ...ready, groqKeySet: false });
  assert.equal(result.canRecord, false);
  assert.equal(result.issue.kind, "speech");
  assert.match(result.issue.title, /Groq key/);
});

test("microphone permission is the first fix when multiple things are missing", () => {
  for (const microphone of ["denied", "restricted", "notDetermined"]) {
    const result = evaluateReadiness({ ...ready, microphone, groqKeySet: false });
    assert.equal(result.canRecord, false);
    assert.equal(result.issue.kind, "microphone");
    assert.equal(result.needsPermission, true);
  }
});

test("local speech checks the engine, download, and model readiness", () => {
  for (const [patch, title] of [
    [{ engineReady: false }, /engine is missing/],
    [{ modelDownloaded: false }, /Download a speech model/],
    [{ modelReady: false }, /model isn't ready/],
  ]) {
    const result = evaluateReadiness({ ...ready, transcription: "local", groqKeySet: false, ...patch });
    assert.equal(result.canRecord, false);
    assert.match(result.issue.title, title);
  }
  const result = evaluateReadiness({ ...ready, transcription: "local", groqKeySet: false });
  assert.equal(result.canRecord, true);
  assert.equal(result.speech, "On this Mac");
});

test("missing paste permission explains clipboard fallback without blocking dictation", () => {
  const result = evaluateReadiness({ ...ready, accessibility: false });
  assert.equal(result.canRecord, true);
  assert.equal(result.issue.kind, "accessibility");
  assert.match(result.issue.description, /Text will be copied/);
});

test("copy-only mode doesn't ask for paste permission", () => {
  const result = evaluateReadiness({ ...ready, accessibility: false, refinementMode: "copy-only" });
  assert.equal(result.canRecord, true);
  assert.equal(result.issue, null);
  assert.equal(result.cleanup, "Off");
  assert.equal(result.needsPermission, false);
});

test("optional clean-up isn't a blocker and its status is accurate", () => {
  const noKey = evaluateReadiness({ ...ready, cleanupProvider: { name: "Groq", needsKey: true, keySet: false } });
  assert.equal(noKey.canRecord, true);
  assert.equal(noKey.issue, null);
  assert.equal(noKey.cleanup, "Off (no key)");
  assert.equal(evaluateReadiness({ ...ready, refinementMode: "raw" }).cleanup, "Off");
  assert.equal(evaluateReadiness({ ...ready, cleanupProvider: { name: "Local", needsKey: false, keySet: false } }).cleanup, "Local");
});
