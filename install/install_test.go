package install

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestSortVersions(t *testing.T) {
	tests := []struct {
		name     string
		versions []string
		want     []string
	}{
		{
			name:     "normal versions",
			versions: []string{"1.0.0", "2.0.0", "1.1.0"},
			want:     []string{"2.0.0", "1.1.0", "1.0.0"},
		},
		{
			name:     "versions with invalid entries",
			versions: []string{"1.0.0", "invalid", "2.0.0"},
			want:     []string{"2.0.0", "1.0.0"},
		},
		{
			name:     "empty list",
			versions: []string{},
			want:     []string{},
		},
		{
			name:     "versions with pre-release",
			versions: []string{"1.0.0", "1.0.0-alpha", "1.0.0-beta"},
			want:     []string{"1.0.0", "1.0.0-beta", "1.0.0-alpha"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := make([]string, len(tt.versions))
			copy(input, tt.versions)
			sortVersions(input)
			if !reflect.DeepEqual(input[:len(tt.want)], tt.want) {
				t.Errorf("sortVersions() = %v, want %v", input, tt.want)
			}
		})
	}
}

func TestResolveVersionInput(t *testing.T) {
	// Save original env and restore after test
	origVersion := os.Getenv("INPUT_GOP_VERSION")
	origFile := os.Getenv("INPUT_GOP_VERSION_FILE")
	defer func() {
		os.Setenv("INPUT_GOP_VERSION", origVersion)
		os.Setenv("INPUT_GOP_VERSION_FILE", origFile)
	}()

	// Create a temporary directory for test files
	tmpDir, err := os.MkdirTemp("", "gop-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	// Create test files
	gopMod := filepath.Join(tmpDir, "gop.mod")
	if err := os.WriteFile(gopMod, []byte("gop 1.2.3\n"), 0644); err != nil {
		t.Fatal(err)
	}

	versionFile := filepath.Join(tmpDir, "version")
	if err := os.WriteFile(versionFile, []byte("1.1.0\n"), 0644); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name        string
		version     string
		versionFile string
		want        string
		wantErr     bool
	}{
		{
			name:    "explicit version",
			version: "1.0.0",
			want:    "1.0.0",
		},
		{
			name:        "version from gop.mod",
			versionFile: gopMod,
			want:        "1.2.3",
		},
		{
			name:        "version from plain file",
			versionFile: versionFile,
			want:        "1.1.0",
		},
		{
			name:        "non-existent file",
			versionFile: "nonexistent",
			wantErr:     true,
		},
		{
			name:        "both inputs specified",
			version:     "1.0.0",
			versionFile: gopMod,
			want:        "1.0.0", // should use version over file
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			os.Setenv("INPUT_GOP_VERSION", tt.version)
			os.Setenv("INPUT_GOP_VERSION_FILE", tt.versionFile)

			got, err := resolveVersionInput()
			if (err != nil) != tt.wantErr {
				t.Errorf("resolveVersionInput() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && got != tt.want {
				t.Errorf("resolveVersionInput() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestParseGopVersionFile(t *testing.T) {
	// Create temporary directory for test files
	tmpDir, err := os.MkdirTemp("", "gop-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	tests := []struct {
		name     string
		content  string
		filename string
		want     string
	}{
		{
			name:     "gop.mod file",
			content:  "gop 1.2.3\nrequire (...)\n",
			filename: "gop.mod",
			want:     "1.2.3",
		},
		{
			name:     "gop.work file",
			content:  "gop 1.1.0\n\nuse ./...\n",
			filename: "gop.work",
			want:     "1.1.0",
		},
		{
			name:     "plain version file",
			content:  "1.0.0\n",
			filename: "version",
			want:     "1.0.0",
		},
		{
			name:     "invalid gop.mod",
			content:  "invalid content",
			filename: "gop.mod",
			want:     "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filePath := filepath.Join(tmpDir, tt.filename)
			if err := os.WriteFile(filePath, []byte(tt.content), 0644); err != nil {
				t.Fatal(err)
			}

			got, err := parseGopVersionFile(filePath)
			if err != nil {
				t.Errorf("parseGopVersionFile() error = %v", err)
				return
			}
			if got != tt.want {
				t.Errorf("parseGopVersionFile() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsValidVersion(t *testing.T) {
	tests := []struct {
		version string
		want    bool
	}{
		{"1.0.0", true},
		{"1.2.3", true},
		{"v1.0.0", true},
		{"1.0", false},
		{"invalid", false},
		{"1.0.0-alpha", true},
		{"1.0.0+build", true},
		{"v1.0.0-beta", true},
		{"1.0.0-beta.1", true},
		{"v1.0.0+001", true},
		{"latest", false},
		{"main", false},
		{"v1.2", false},
		{"v1", false},
	}

	for _, tt := range tests {
		t.Run(tt.version, func(t *testing.T) {
			if got := isValidVersion(tt.version); got != tt.want {
				t.Errorf("isValidVersion(%q) = %v, want %v", tt.version, got, tt.want)
			}
		})
	}
}

func TestIsValidVersionConstraint(t *testing.T) {
	tests := []struct {
		constraint string
		want       bool
	}{
		{"1.0.0", true},
		{"1.0", true},
		{"v1", true},
		{">=1.0.0", true},
		{">1.0", true},
		{"~1.0.0", true},
		{"^1.0.0", true},
		{"1.0.x", true},
		{"invalid", false},
		{"latest", false},
		{"main", false},
	}

	for _, tt := range tests {
		t.Run(tt.constraint, func(t *testing.T) {
			if got := isValidVersionConstraint(tt.constraint); got != tt.want {
				t.Errorf("isValidVersionConstraint(%q) = %v, want %v", tt.constraint, got, tt.want)
			}
		})
	}
}

func TestMaxSatisfying(t *testing.T) {
	tests := []struct {
		name     string
		versions []string
		spec     string
		want     string
	}{
		{
			name:     "exact version",
			versions: []string{"1.0.0", "1.1.0", "2.0.0"},
			spec:     "1.0.0",
			want:     "1.0.0",
		},
		{
			name:     "greater than",
			versions: []string{"1.0.0", "1.1.0", "2.0.0"},
			spec:     ">1.0.0",
			want:     "2.0.0",
		},
		{
			name:     "version range",
			versions: []string{"1.0.0", "1.1.0", "2.0.0"},
			spec:     ">=1.0.0 <2.0.0",
			want:     "1.1.0",
		},
		{
			name:     "no matching version",
			versions: []string{"1.0.0", "1.1.0", "2.0.0"},
			spec:     "3.0.0",
			want:     "",
		},
		{
			name:     "invalid constraint",
			versions: []string{"1.0.0", "1.1.0", "2.0.0"},
			spec:     "invalid",
			want:     "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := maxSatisfying(tt.versions, tt.spec); got != tt.want {
				t.Errorf("maxSatisfying() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCheckVersion(t *testing.T) {
	// Save original function and restore after test
	originalFunc := gopVersionFunc
	defer func() { gopVersionFunc = originalFunc }()

	tests := []struct {
		name        string
		versionSpec string
		mockVersion string
		wantErr     bool
	}{
		{
			name:        "matching versions",
			versionSpec: "1.0.0",
			mockVersion: "1.0.0",
			wantErr:     false,
		},
		{
			name:        "mismatched versions",
			versionSpec: "1.0.0",
			mockVersion: "1.1.0",
			wantErr:     true,
		},
		{
			name:        "invalid spec version",
			versionSpec: "invalid",
			mockVersion: "1.0.0",
			wantErr:     true,
		},
		{
			name:        "invalid installed version",
			versionSpec: "1.0.0",
			mockVersion: "invalid",
			wantErr:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set mock function for this test
			gopVersionFunc = func() (string, error) {
				return tt.mockVersion, nil
			}

			err := checkVersion(tt.versionSpec)
			if (err != nil) != tt.wantErr {
				t.Errorf("checkVersion() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
