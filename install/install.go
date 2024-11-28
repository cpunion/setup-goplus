package install

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/Masterminds/semver/v3"
)

const GoplusRepo = "https://github.com/goplus/gop.git"

var gopVersionFunc = getGopVersion

// InstallGop is the main function for installing gop
func InstallGop() error {
	versionSpec, err := resolveVersionInput()
	if err != nil {
		return err
	}

	tagVersions, err := fetchTags()
	if err != nil {
		return err
	}

	// Filter and sort valid versions
	var validVersions []string
	for _, v := range tagVersions {
		if isValidVersion(v) {
			validVersions = append(validVersions, v)
		}
	}
	sortVersions(validVersions)

	var version string
	if versionSpec == "" || versionSpec == "latest" {
		version = validVersions[0]
		warning(fmt.Sprintf("No gop-version specified, using latest version: %s", version))
	} else {
		version = maxSatisfying(validVersions, versionSpec)
		if version == "" {
			warning(fmt.Sprintf("No gop-version found that satisfies '%s', trying branches...", versionSpec))
			branches, err := fetchBranches()
			if err != nil {
				return err
			}
			if !contains(branches, versionSpec) {
				return fmt.Errorf("no gop-version found that satisfies '%s' in branches or tags", versionSpec)
			}
		}
	}

	var checkoutVersion string
	if version != "" {
		info(fmt.Sprintf("Selected version %s by spec %s", version, versionSpec))
		checkoutVersion = "v" + version
		setOutput("gop-version-verified", "true")
	} else {
		warning(fmt.Sprintf("Unable to find a version that satisfies the version spec '%s', trying branches...", versionSpec))
		checkoutVersion = versionSpec
		setOutput("gop-version-verified", "false")
	}

	gopDir, err := cloneBranchOrTag(checkoutVersion)
	if err != nil {
		return err
	}

	if err := install(gopDir); err != nil {
		return err
	}

	if version != "" {
		if err := checkVersion(version); err != nil {
			return err
		}
	}

	gopVersion, err := gopVersionFunc()
	if err != nil {
		return err
	}
	setOutput("gop-version", gopVersion)

	return nil
}

func sortVersions(versions []string) {
	// Convert string versions to semver.Version objects
	vs := make([]*semver.Version, 0, len(versions))
	for _, raw := range versions {
		v, err := semver.NewVersion(raw)
		if err != nil {
			continue // Skip invalid versions
		}
		vs = append(vs, v)
	}

	// Sort versions in descending order (newest first)
	sort.Sort(sort.Reverse(semver.Collection(vs)))

	// Convert back to strings
	for i, v := range vs {
		versions[i] = v.String()
	}
}

// isValidVersion checks if a version string is a valid complete semver version
func isValidVersion(version string) bool {
	// First check if it matches X.Y.Z pattern (with optional v prefix)
	version = strings.TrimPrefix(version, "v")
	parts := strings.Split(strings.Split(version, "-")[0], ".")
	if len(parts) != 3 {
		return false
	}

	// Then validate with semver library
	_, err := semver.NewVersion(version)
	return err == nil
}

// isValidVersionConstraint checks if a version string is a valid version constraint
// This is used for validating user input version specifications
func isValidVersionConstraint(version string) bool {
	_, err := semver.NewConstraint(version)
	return err == nil
}

// maxSatisfying finds the highest version that satisfies the constraint
func maxSatisfying(versions []string, spec string) string {
	// First try exact version match
	for _, v := range versions {
		if v == strings.TrimPrefix(spec, "v") {
			return v
		}
	}

	// Then try as a constraint
	c, err := semver.NewConstraint(spec)
	if err != nil {
		return "" // Invalid constraint
	}

	var maxVersion *semver.Version
	for _, raw := range versions {
		v, err := semver.NewVersion(raw)
		if err != nil {
			continue
		}

		if c.Check(v) {
			if maxVersion == nil || v.GreaterThan(maxVersion) {
				maxVersion = v
			}
		}
	}

	if maxVersion == nil {
		return ""
	}
	return maxVersion.String()
}

func cloneBranchOrTag(versionSpec string) (string, error) {
	workDir := filepath.Join(os.Getenv("HOME"), "workdir")
	if err := os.RemoveAll(workDir); err != nil {
		return "", err
	}
	if err := os.MkdirAll(workDir, 0755); err != nil {
		return "", err
	}

	info(fmt.Sprintf("Cloning gop %s to %s ...", versionSpec, workDir))
	cmd := exec.Command("git", "clone", "--depth", "1", "--branch", versionSpec, GoplusRepo)
	cmd.Dir = workDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return "", err
	}
	info("gop cloned")
	return filepath.Join(workDir, "gop"), nil
}

func install(gopDir string) error {
	info(fmt.Sprintf("Installing gop %s ...", gopDir))
	binDir := filepath.Join(os.Getenv("HOME"), "bin")
	cmd := exec.Command("go", "run", "cmd/make.go", "-install")
	cmd.Dir = gopDir
	cmd.Env = append(os.Environ(), "GOBIN="+binDir)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return err
	}

	addToPath(binDir)
	info("gop installed")
	return nil
}

func checkVersion(versionSpec string) error {
	info(fmt.Sprintf("Testing gop %s ...", versionSpec))
	actualVersion, err := gopVersionFunc()
	if err != nil {
		return err
	}

	// Parse versions using semver
	expected, err := semver.NewVersion(versionSpec)
	if err != nil {
		return fmt.Errorf("invalid version spec: %s", versionSpec)
	}

	actual, err := semver.NewVersion(actualVersion)
	if err != nil {
		return fmt.Errorf("invalid installed version: %s", actualVersion)
	}

	if !expected.Equal(actual) {
		return fmt.Errorf("installed gop version %s does not match expected version %s",
			actual.String(), expected.String())
	}

	info(fmt.Sprintf("Installed gop version %s", actualVersion))
	return nil
}

func getGopVersion() (string, error) {
	cmd := exec.Command("gop", "env", "GOPVERSION")
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to get gop version: %w", err)
	}
	return strings.TrimPrefix(strings.TrimSpace(string(out)), "v"), nil
}

func fetchTags() ([]string, error) {
	cmd := exec.Command("git", "-c", "versionsort.suffix=-", "ls-remote", "--tags", "--sort=v:refname", GoplusRepo)
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	var versions []string
	for _, line := range strings.Split(string(out), "\n") {
		if line == "" {
			continue
		}
		parts := strings.Split(line, "\t")
		version := strings.TrimPrefix(strings.TrimPrefix(parts[1], "refs/tags/"), "v")
		versions = append(versions, version)
	}
	return versions, nil
}

func fetchBranches() ([]string, error) {
	cmd := exec.Command("git", "-c", "versionsort.suffix=-", "ls-remote", "--heads", "--sort=v:refname", GoplusRepo)
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	var branches []string
	for _, line := range strings.Split(string(out), "\n") {
		if line == "" {
			continue
		}
		parts := strings.Split(line, "\t")
		branch := strings.TrimPrefix(parts[1], "refs/heads/")
		branches = append(branches, branch)
	}
	return branches, nil
}

func resolveVersionInput() (string, error) {
	version := os.Getenv("INPUT_GOP_VERSION")
	versionFile := os.Getenv("INPUT_GOP_VERSION_FILE")

	if version != "" && versionFile != "" {
		warning("Both gop-version and gop-version-file inputs are specified, only gop-version will be used")
		return version, nil
	}

	if version != "" {
		return version, nil
	}

	if versionFile != "" {
		if _, err := os.Stat(versionFile); os.IsNotExist(err) {
			return "", fmt.Errorf("the specified gop version file at: %s does not exist", versionFile)
		}
		return parseGopVersionFile(versionFile)
	}

	return "", nil
}

func parseGopVersionFile(versionFilePath string) (string, error) {
	content, err := os.ReadFile(versionFilePath)
	if err != nil {
		return "", err
	}

	filename := filepath.Base(versionFilePath)
	if filename == "gop.mod" || filename == "gop.work" {
		re := regexp.MustCompile(`^gop (\d+(\.\d+)*)`)
		match := re.FindSubmatch(content)
		if match != nil {
			return string(match[1]), nil
		}
		return "", nil
	}

	return strings.TrimSpace(string(content)), nil
}

func setOutput(name, value string) {
	if outputFile := os.Getenv("GITHUB_OUTPUT"); outputFile != "" {
		f, _ := os.OpenFile(outputFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		fmt.Fprintf(f, "%s=%s\n", name, value)
		f.Close()
	} else {
		panic(fmt.Sprintf("GITHUB_OUTPUT is not set, cannot set output %s=%s", name, value))
	}
}

func addToPath(path string) {
	if pathFile := os.Getenv("GITHUB_PATH"); pathFile != "" {
		f, _ := os.OpenFile(pathFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		fmt.Fprintln(f, path)
		f.Close()
	} else {
		panic("GITHUB_PATH is not set, cannot add to PATH")
	}
	os.Setenv("PATH", path+":"+os.Getenv("PATH"))
}

func info(msg string) {
	fmt.Println(msg)
}

func warning(msg string) {
	fmt.Printf("::warning::%s\n", msg)
}

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}
