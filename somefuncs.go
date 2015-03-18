package installasservice

import (
	"os/exec"
	"regexp"
	"runtime"
	"strings"
	"syscall"
)

func RegexReplace(source, rx, repl string) (string, error) {
	r, err := regexp.Compile(rx)
	if err != nil {
		return "", err
	}
	res := r.ReplaceAllString(source, repl)
	return res, nil
}

func sliceToCmdStr(args []string) string {
	s := ""
	for _, v := range args {
		hasspace := false
		for _, v2 := range v {
			if v2 == ' ' {
				hasspace = true
				break
			}
		}
		if hasspace {
			switch runtime.GOOS {
			case "windows":
				v = "\"" + strings.Replace(v, "\"", "\\\"", -1) + "\""
			default:
				v = "\"" + strings.Replace(v, "\"", "\"\"", -1) + "\""
			}
		}
		s += " " + v
	}
	return s
}

func getExitCode(err error) int {
	if exiterr, ok := err.(*exec.ExitError); ok {
		// The program has exited with an exit code != 0

		// This works on both Unix and Windows. Although package
		// syscall is generally platform dependent, WaitStatus is
		// defined for both Unix and Windows and in both cases has
		// an ExitStatus() method with the same signature.
		if status, ok := exiterr.Sys().(syscall.WaitStatus); ok {
			return status.ExitStatus()
		}
		return -1
	}
	return 0
}

func Bold(str string) string {
	if runtime.GOOS == "windows" {
		return str
	}
	return "\033[1m" + str + "\033[0m"
}
