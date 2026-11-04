package speckit

import (
	"os"
	"os/exec"
	"path/filepath"
)

const specKitRepo = "git+https://github.com/github/spec-kit.git"

// InstallHint is the recommended way to install the specify CLI.
const InstallHint = "curl -LsSf https://astral.sh/uv/install.sh | sh && uv tool install specify-cli --from " + specKitRepo

// SpecifyCommand finds a way to run the `specify` CLI: an installed binary,
// or an ephemeral run through uvx / pipx. It also looks in the usual install
// dirs (~/.local/bin, ~/.cargo/bin) in case they are not on PATH.
// It returns the executable and the args that must precede specify's own.
func SpecifyCommand() (bin string, prefix []string, ok bool) {
	if p := lookPath("specify"); p != "" {
		return p, nil, true
	}
	if p := lookPath("uvx"); p != "" {
		return p, []string{"--from", specKitRepo, "specify"}, true
	}
	if p := lookPath("uv"); p != "" {
		return p, []string{"tool", "run", "--from", specKitRepo, "specify"}, true
	}
	if p := lookPath("pipx"); p != "" {
		return p, []string{"run", "--spec", specKitRepo, "specify"}, true
	}
	return "", nil, false
}

func lookPath(name string) string {
	if p, err := exec.LookPath(name); err == nil {
		return p
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	for _, dir := range []string{".local/bin", ".cargo/bin"} {
		p := filepath.Join(home, dir, name)
		if st, err := os.Stat(p); err == nil && !st.IsDir() && st.Mode()&0o111 != 0 {
			return p
		}
	}
	return ""
}
