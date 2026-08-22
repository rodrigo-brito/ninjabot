package ui

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime/debug"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// makeTarGz builds a tar.gz with the given files.
func makeTarGz(t *testing.T, files map[string]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	for name, content := range files {
		require.NoError(t, tw.WriteHeader(&tar.Header{Name: name, Mode: 0o644, Size: int64(len(content)), Typeflag: tar.TypeReg}))
		_, err := tw.Write([]byte(content))
		require.NoError(t, err)
	}
	require.NoError(t, tw.Close())
	require.NoError(t, gz.Close())
	return buf.Bytes()
}

func sha256Hex(b []byte) string {
	sum := sha256.New()
	sum.Write(b)
	return hex.EncodeToString(sum.Sum(nil))
}

// fakeReleases emulates github.com/<owner>/<repo>/releases.
type fakeReleases struct {
	t        *testing.T
	latest   string
	bundles  map[string][]byte // tag -> tar.gz
	checksum map[string]string // tag -> checksums.txt body
	hits     atomic.Int32
	server   *httptest.Server
}

func newFakeReleases(t *testing.T, latest string, bundles map[string][]byte) *fakeReleases {
	t.Helper()
	f := &fakeReleases{t: t, latest: latest, bundles: bundles, checksum: map[string]string{}}
	for tag, b := range bundles {
		f.checksum[tag] = fmt.Sprintf("%s  ninjabot_Linux_x86_64.tar.gz\n%s  %s\n", sha256Hex([]byte("other")), sha256Hex(b), bundleAsset)
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/releases/latest/download/", func(w http.ResponseWriter, r *http.Request) {
		f.hits.Add(1)
		asset := filepath.Base(r.URL.Path)
		http.Redirect(w, r, fmt.Sprintf("%s/releases/download/%s/%s", f.server.URL, f.latest, asset), http.StatusFound)
	})
	mux.HandleFunc("/releases/download/", func(w http.ResponseWriter, r *http.Request) {
		f.hits.Add(1)
		// /releases/download/<tag>/<asset>
		tag := filepath.Base(filepath.Dir(r.URL.Path))
		asset := filepath.Base(r.URL.Path)
		switch asset {
		case bundleAsset:
			if b, ok := f.bundles[tag]; ok {
				_, _ = w.Write(b)
				return
			}
		case checksumsAsset:
			if c, ok := f.checksum[tag]; ok {
				_, _ = w.Write([]byte(c))
				return
			}
		}
		http.NotFound(w, r)
	})
	f.server = httptest.NewServer(mux)
	t.Cleanup(f.server.Close)
	return f
}

func (f *fakeReleases) config(cacheDir string) bundleConfig {
	cfg := defaultBundleConfig()
	cfg.dir = ""
	cfg.version = ""
	cfg.cacheDir = cacheDir
	cfg.releaseURL = f.server.URL + "/releases"
	return cfg
}

func TestResolveBundle_DownloadsVerifiesAndCaches(t *testing.T) {
	t.Setenv(envUIDir, "")
	t.Setenv(envUIVersion, "")

	good := makeTarGz(t, map[string]string{"app.js": "console.log(1)", "app.css": "body{}", "assets/x.txt": "x"})
	releases := newFakeReleases(t, "v1.2.0", map[string][]byte{"v1.2.0": good})
	cacheDir := t.TempDir()

	cfg := releases.config(cacheDir)
	cfg.version = "v1.2.0"

	b, err := resolveBundle(context.Background(), cfg)
	require.NoError(t, err)
	assert.Equal(t, "v1.2.0", b.version)
	assert.Equal(t, filepath.Join(cacheDir, "v1.2.0"), b.dir)
	assert.FileExists(t, filepath.Join(b.dir, "app.js"))
	assert.FileExists(t, filepath.Join(b.dir, "assets", "x.txt"))
	assert.FileExists(t, filepath.Join(b.dir, completeMarker))
	assert.Equal(t, int32(2), releases.hits.Load(), "checksums + bundle")

	// second call is served from cache without touching the network
	b2, err := resolveBundle(context.Background(), cfg)
	require.NoError(t, err)
	assert.Equal(t, b, b2)
	assert.Equal(t, int32(2), releases.hits.Load())

	// no temp files left behind
	entries, err := os.ReadDir(cacheDir)
	require.NoError(t, err)
	require.Len(t, entries, 1)
}

func TestResolveBundle_Latest(t *testing.T) {
	good := makeTarGz(t, map[string]string{"app.js": "1"})
	releases := newFakeReleases(t, "v2.0.0", map[string][]byte{"v2.0.0": good})
	cfg := releases.config(t.TempDir())
	cfg.version = versionLatest

	b, err := resolveBundle(context.Background(), cfg)
	require.NoError(t, err)
	assert.Equal(t, "v2.0.0", b.version)
}

func TestResolveBundle_ChecksumMismatch(t *testing.T) {
	good := makeTarGz(t, map[string]string{"app.js": "1"})
	releases := newFakeReleases(t, "v1.0.0", map[string][]byte{"v1.0.0": good})
	releases.checksum["v1.0.0"] = sha256Hex([]byte("tampered")) + "  " + bundleAsset + "\n"
	cacheDir := t.TempDir()
	cfg := releases.config(cacheDir)
	cfg.version = "v1.0.0"

	_, err := resolveBundle(context.Background(), cfg)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "checksum mismatch")
	assert.NoDirExists(t, filepath.Join(cacheDir, "v1.0.0"))
}

func TestResolveBundle_MissingChecksumEntry(t *testing.T) {
	good := makeTarGz(t, map[string]string{"app.js": "1"})
	releases := newFakeReleases(t, "v1.0.0", map[string][]byte{"v1.0.0": good})
	releases.checksum["v1.0.0"] = "abc  something-else.tar.gz\n"
	cfg := releases.config(t.TempDir())
	cfg.version = "v1.0.0"

	_, err := resolveBundle(context.Background(), cfg)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no entry")
}

func TestResolveBundle_BundleWithoutAppJS(t *testing.T) {
	bad := makeTarGz(t, map[string]string{"index.html": "1"})
	releases := newFakeReleases(t, "v1.0.0", map[string][]byte{"v1.0.0": bad})
	cfg := releases.config(t.TempDir())
	cfg.version = "v1.0.0"

	_, err := resolveBundle(context.Background(), cfg)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "does not contain app.js")
}

func TestResolveBundle_UnknownVersion(t *testing.T) {
	releases := newFakeReleases(t, "v1.0.0", map[string][]byte{})
	cfg := releases.config(t.TempDir())
	cfg.version = "v9.9.9"

	_, err := resolveBundle(context.Background(), cfg)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "404")
}

func TestResolveBundle_OfflineFallsBackToCache(t *testing.T) {
	good := makeTarGz(t, map[string]string{"app.js": "1"})
	releases := newFakeReleases(t, "v1.0.0", map[string][]byte{"v1.0.0": good, "v1.1.0": good})
	cacheDir := t.TempDir()

	for _, v := range []string{"v1.0.0", "v1.1.0"} {
		cfg := releases.config(cacheDir)
		cfg.version = v
		_, err := resolveBundle(context.Background(), cfg)
		require.NoError(t, err)
	}

	releases.server.Close() // go offline
	cfg := releases.config(cacheDir)
	cfg.version = versionLatest

	b, err := resolveBundle(context.Background(), cfg)
	require.NoError(t, err)
	assert.Equal(t, "v1.1.0", b.version, "newest cached version wins")
}

func TestResolveBundle_OfflineWithoutCache(t *testing.T) {
	releases := newFakeReleases(t, "v1.0.0", map[string][]byte{})
	releases.server.Close()
	cfg := releases.config(t.TempDir())
	cfg.version = "v1.0.0"

	_, err := resolveBundle(context.Background(), cfg)
	require.Error(t, err)
}

func TestResolveBundle_LocalDir(t *testing.T) {
	dir := t.TempDir()
	cfg := bundleConfig{dir: dir}

	_, err := resolveBundle(context.Background(), cfg)
	require.Error(t, err, "missing app.js")

	require.NoError(t, os.WriteFile(filepath.Join(dir, "app.js"), []byte("1"), 0o644))
	b, err := resolveBundle(context.Background(), cfg)
	require.NoError(t, err)
	assert.Equal(t, versionLocal, b.version)
	assert.Equal(t, dir, b.dir)
}

func TestResolveBundle_EnvOverrides(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "app.js"), []byte("1"), 0o644))
	t.Setenv(envUIDir, dir)
	t.Setenv(envUIVersion, "v0.0.1")

	cfg := defaultBundleConfig()
	assert.Equal(t, dir, cfg.dir)
	assert.Equal(t, "v0.0.1", cfg.version)

	c, err := New()
	require.NoError(t, err)
	assert.Equal(t, dir, c.bundle.dir)

	c, err = New(WithUIDir("/elsewhere"), WithUIVersion("v1.0.0"), WithCacheDir("/cache"), WithHTTPClient(http.DefaultClient), withReleaseURL("http://x"))
	require.NoError(t, err)
	assert.Equal(t, "/elsewhere", c.bundle.dir)
	assert.Equal(t, "v1.0.0", c.bundle.version)
	assert.Equal(t, "/cache", c.bundle.cacheDir)
	assert.Equal(t, http.DefaultClient, c.bundle.client)
	assert.Equal(t, "http://x", c.bundle.releaseURL)
}

func TestExtractTarGz_RejectsPathTraversal(t *testing.T) {
	evil := makeTarGz(t, map[string]string{"../escape.js": "1"})
	err := extractTarGz(bytes.NewReader(evil), t.TempDir())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "illegal path")

	abs := makeTarGz(t, map[string]string{"/etc/passwd": "1"})
	err = extractTarGz(bytes.NewReader(abs), t.TempDir())
	require.Error(t, err)
}

func TestIsReleaseTag(t *testing.T) {
	assert.True(t, isReleaseTag("v1.2.3"))
	assert.True(t, isReleaseTag("v1.2.3-rc.1"))
	assert.False(t, isReleaseTag("1.2.3"))
	assert.False(t, isReleaseTag("latest"))
	assert.False(t, isReleaseTag("v0.0.0-20260822101010-abcdef123456"))
	assert.False(t, isReleaseTag("v0.5.1-0.20260822101010-abcdef123456"))
	assert.False(t, isReleaseTag("v1.2.0-rc.1.0.20260822101010-abcdef123456"))
	assert.False(t, isReleaseTag("v1.0.0+incompatible"))
	assert.False(t, isReleaseTag("v1.2.3+meta"))
	assert.False(t, isReleaseTag("(devel)"))
}

func TestDetectVersionFrom(t *testing.T) {
	dep := func(version string, replace *debug.Module) *debug.Module {
		return &debug.Module{Path: modulePath, Version: version, Replace: replace}
	}
	consumer := func(deps ...*debug.Module) *debug.BuildInfo {
		return &debug.BuildInfo{Main: debug.Module{Path: "example.com/bot"}, Deps: deps}
	}
	repo := func(version string) *debug.BuildInfo {
		return &debug.BuildInfo{Main: debug.Module{Path: modulePath, Version: version}}
	}
	describeTag := func(tag string) func(string) string {
		return func(string) string { return tag }
	}
	noDescribe := func(dir string) string {
		t.Errorf("git describe must not run for %s", dir)
		return ""
	}

	t.Run("exact module version", func(t *testing.T) {
		d := detectVersionFrom(consumer(dep("v1.4.0", nil)), "/src/ui", noDescribe)
		assert.Equal(t, detectedVersion{version: "v1.4.0", reason: "module version"}, d)
	})

	t.Run("pseudo-version falls back to the release before it", func(t *testing.T) {
		d := detectVersionFrom(consumer(dep("v0.5.1-0.20260822101010-abcdef123456", nil)), "", noDescribe)
		assert.Equal(t, "v0.5.0", d.version)
		assert.True(t, d.inferred)
	})

	t.Run("pseudo-version without a previous tag", func(t *testing.T) {
		d := detectVersionFrom(consumer(dep("v0.0.0-20260822101010-abcdef123456", nil)), "", describeTag("v9.9.9"))
		assert.Equal(t, "", d.version, "consumers never use git describe")
	})

	t.Run("not a dependency", func(t *testing.T) {
		d := detectVersionFrom(consumer(), "/src/ui", noDescribe)
		assert.Equal(t, "", d.version)
		assert.Contains(t, d.reason, "not a dependency")
	})

	t.Run("replace to a local directory uses git describe", func(t *testing.T) {
		var seen string
		describe := func(dir string) string { seen = dir; return "v0.5.0" }
		d := detectVersionFrom(consumer(dep("v0.0.0-00010101000000-000000000000", &debug.Module{Path: "../ninjabot"})), "", describe)
		assert.Equal(t, "../ninjabot", seen)
		assert.Equal(t, "v0.5.0", d.version)
		assert.True(t, d.inferred)

		d = detectVersionFrom(consumer(dep("v0.5.0", &debug.Module{Path: "../ninjabot"})), "", describeTag(""))
		assert.Equal(t, "", d.version, "no tags in the checkout")
	})

	t.Run("replace to another module version does not describe", func(t *testing.T) {
		d := detectVersionFrom(consumer(dep("v0.5.0", &debug.Module{Path: "github.com/fork/ninjabot", Version: "v0.5.0"})), "", noDescribe)
		assert.Equal(t, "", d.version)
	})

	t.Run("built inside the repository", func(t *testing.T) {
		// go build stamps a VCS pseudo-version (Go 1.24+)
		d := detectVersionFrom(repo("v0.5.1-0.20260822143150-a993611026c5+dirty"), "/src/ui", noDescribe)
		assert.Equal(t, "v0.5.0", d.version)

		// go run reports (devel): ask git
		var seen string
		describe := func(dir string) string { seen = dir; return "v0.5.0" }
		d = detectVersionFrom(repo("(devel)"), "/src/ui", describe)
		assert.Equal(t, "/src/ui", seen)
		assert.Equal(t, "v0.5.0", d.version)
		assert.True(t, d.inferred)

		// exact tag checkout
		d = detectVersionFrom(repo("v0.5.0"), "/src/ui", noDescribe)
		assert.Equal(t, detectedVersion{version: "v0.5.0", reason: "module version"}, d)

		// trimpath / moved binary: no source dir, no git
		d = detectVersionFrom(repo("(devel)"), "", noDescribe)
		assert.Equal(t, "", d.version)
	})
}

func TestBaseOfPseudoVersion(t *testing.T) {
	cases := map[string]string{
		"v0.5.1-0.20260822101010-abcdef123456":       "v0.5.0",
		"v0.5.1-0.20260822101010-abcdef123456+dirty": "v0.5.0",
		"v1.0.0-0.20260822101010-abcdef123456":       "", // v1.0.0-0 cannot follow a tag: patch is 0
		"v0.0.0-20260822101010-abcdef123456":         "",
		"v1.2.0-rc.1.0.20260822101010-abcdef123456":  "v1.2.0-rc.1",
		"v1.2.3":                                "",
		"latest":                                "",
		"(devel)":                               "",
		"v2.10.4-0.20260822101010-abcdef123456": "v2.10.3",
	}
	for in, want := range cases {
		assert.Equal(t, want, baseOfPseudoVersion(in), in)
	}
}

func TestGitDescribe(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not installed")
	}
	dir := t.TempDir()
	run := func(args ...string) {
		cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
		cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t")
		out, err := cmd.CombinedOutput()
		require.NoError(t, err, string(out))
	}
	run("init", "-q")
	assert.Equal(t, "", gitDescribe(dir), "no commits")

	run("commit", "-q", "--allow-empty", "-m", "one")
	assert.Equal(t, "", gitDescribe(dir), "no tags")

	run("tag", "v0.3.0")
	run("tag", "not-a-release")
	run("commit", "-q", "--allow-empty", "-m", "two")
	run("tag", "v0.4.0-rc.1")
	run("commit", "-q", "--allow-empty", "-m", "three")
	assert.Equal(t, "v0.4.0-rc.1", gitDescribe(dir))

	assert.Equal(t, "", gitDescribe(filepath.Join(t.TempDir(), "missing")))
}

func TestNewestCached(t *testing.T) {
	cacheDir := t.TempDir()
	_, ok := newestCached(cacheDir)
	assert.False(t, ok)

	for _, v := range []string{"v1.0.0", "v1.10.0", "v1.9.0", "garbage"} {
		require.NoError(t, os.MkdirAll(filepath.Join(cacheDir, v), 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(cacheDir, v, completeMarker), nil, 0o644))
	}
	require.NoError(t, os.MkdirAll(filepath.Join(cacheDir, "v2.0.0"), 0o755)) // incomplete

	b, ok := newestCached(cacheDir)
	require.True(t, ok)
	assert.Equal(t, "v1.10.0", b.version)
}
