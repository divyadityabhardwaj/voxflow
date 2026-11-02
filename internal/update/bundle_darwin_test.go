package update

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// Exercise the actual macOS release archive format and ad-hoc signature after
// extraction, rather than just trusting that our ZIP reader preserves modes.
func TestSignedReleaseArchive(t *testing.T) {
	dir := t.TempDir()
	app := filepath.Join(dir, "voxflow.app")
	macos := filepath.Join(app, "Contents", "MacOS")
	if err := os.MkdirAll(macos, 0755); err != nil {
		t.Fatal(err)
	}
	plist := `<?xml version="1.0"?><plist version="1.0"><dict>
<key>CFBundlePackageType</key><string>APPL</string>
<key>CFBundleIdentifier</key><string>com.wails.voxflow</string>
<key>CFBundleExecutable</key><string>voxflow</string>
<key>CFBundleShortVersionString</key><string>1.2.3</string>
</dict></plist>`
	if err := os.WriteFile(filepath.Join(app, "Contents", "Info.plist"), []byte(plist), 0644); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("cc", "-x", "c", "-o", filepath.Join(macos, "voxflow"), "-")
	cmd.Stdin = strings.NewReader("int main(void) { return 0; }")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("compile fixture: %v: %s", err, out)
	}
	for _, name := range []string{"voxflow-updater", "whisper-cli", "whisper-server"} {
		if err := copyHelper(filepath.Join(macos, "voxflow"), filepath.Join(macos, name)); err != nil {
			t.Fatal(err)
		}
	}
	if out, err := exec.Command("/usr/bin/codesign", "--force", "--deep", "--sign", "-", app).CombinedOutput(); err != nil {
		t.Fatalf("sign fixture: %v: %s", err, out)
	}
	archive := filepath.Join(dir, "update.zip")
	if out, err := exec.Command("/usr/bin/ditto", "-c", "-k", "--keepParent", "--norsrc", "--noextattr", app, archive).CombinedOutput(); err != nil {
		t.Fatalf("archive fixture: %v: %s", err, out)
	}
	extracted := t.TempDir()
	if err := extract(context.Background(), archive, extracted); err != nil {
		t.Fatal(err)
	}
	extractedApp := filepath.Join(extracted, "voxflow.app")
	if err := verifyBundle(context.Background(), extractedApp, "1.2.3"); err != nil {
		t.Fatal(err)
	}
	if err := verifyBundle(context.Background(), extractedApp, "9.9.9"); err == nil {
		t.Fatal("accepted an unexpected version")
	}
	if err := os.WriteFile(filepath.Join(extractedApp, "Contents/MacOS/voxflow-updater"), []byte("tampered"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := verifyBundle(context.Background(), extractedApp, "1.2.3"); err == nil {
		t.Fatal("accepted a broken code signature")
	}
}
