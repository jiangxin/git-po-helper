package util

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"

	log "github.com/sirupsen/logrus"
)

// FileRevision is used as an argument for diff function
type FileRevision struct {
	Revision string
	File     string
	Tmpfile  string
}

// Example output:
//
//	git.pot:NNN: this message is used but not defined in /tmp/git.po.XXXX
//	/tmp/git.po.XXXX:NNN: warning: this message is not used
var (
	reNewEntry = regexp.MustCompile(`:([0-9]*): this message is used but not defined in`)
	reDelEntry = regexp.MustCompile(`:([0-9]*): warning: this message is not used`)
)

func checkoutTmpfile(f *FileRevision) error {
	if f.Tmpfile == "" {
		tmpfile, err := os.CreateTemp("", "*--"+filepath.Base(f.File))
		if err != nil {
			return fmt.Errorf("fail to create tmpfile: %s", err)
		}
		f.Tmpfile = tmpfile.Name()
		tmpfile.Close()
	}
	if f.Revision == "" {
		// Read file from f.File and write to f.Tmpfile
		data, err := os.ReadFile(f.File)
		if err != nil {
			return fmt.Errorf("fail to read file: %w", err)
		}
		if err := os.WriteFile(f.Tmpfile, data, 0644); err != nil {
			return fmt.Errorf("fail to write tmpfile: %w", err)
		}
		log.Debugf("read file %s from %s and write to %s", f.File, f.Revision, f.Tmpfile)
		return nil
	}
	cmd := exec.Command("git",
		"show",
		f.Revision+":"+f.File)
	cmd.Stderr = os.Stderr
	out, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf(`get StdoutPipe failed: %s`, err)
	}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("fail to start git-show command: %s", err)
	}
	data, err := io.ReadAll(out)
	out.Close()
	if err != nil {
		return fmt.Errorf("fail to read git-show output: %w", err)
	}
	if err := cmd.Wait(); err != nil {
		return fmt.Errorf("fail to wait git-show command: %s", err)
	}
	if err := os.WriteFile(f.Tmpfile, data, 0644); err != nil {
		return fmt.Errorf("fail to write tmpfile: %w", err)
	}
	log.Debugf(`creating "%s" file using command: %s`, f.Tmpfile, cmd.String())
	return nil
}

// PoFileRevisionDiffStat implements diff on two files with specific revision.
func PoFileRevisionDiffStat(src, dest FileRevision) bool {
	var (
		srcFile  string
		destFile string
	)
	if err := checkoutTmpfile(&src); err != nil {
		log.Errorf("fail to checkout %s of revision %s: %s", src.File, src.Revision, err)
	}
	if err := checkoutTmpfile(&dest); err != nil {
		log.Errorf("fail to checkout %s of revision %s: %s", dest.File, dest.Revision, err)
	}
	if src.Tmpfile != "" {
		srcFile = src.Tmpfile
		defer func() {
			os.Remove(src.Tmpfile)
			src.Tmpfile = ""
		}()
	} else {
		srcFile = src.File
	}
	if dest.Tmpfile != "" {
		destFile = dest.Tmpfile
		defer func() {
			os.Remove(dest.Tmpfile)
			dest.Tmpfile = ""
		}()
	} else {
		destFile = dest.File
	}
	return PoFileDiffStat(srcFile, destFile)
}

// PoFileDiffStat implements diff on two files.
func PoFileDiffStat(src string, dest string) bool {
	var (
		add int32
		del int32
	)
	if !Exist(src) {
		log.Fatalf(`file "%s" not exist`, src)
	}
	if !Exist(dest) {
		log.Fatalf(`file "%s" not exist`, dest)
	}

	cmd := exec.Command("msgcmp",
		"-N",
		"--use-untranslated",
		src,
		dest)

	cmd.Env = os.Environ()
	cmd.Env = append(cmd.Env, "LANGUAGE=C")
	out, err := cmd.StderrPipe()
	if err != nil {
		log.Fatalf("fail to run msgcmp: %s", err)
	}
	log.Debugf("running diff command: %s", cmd.String())
	if err := cmd.Start(); err != nil {
		log.Fatalf("fail to start msgcmp: %s", err)
	}
	reader := bufio.NewReader(out)
	for {
		line, err := reader.ReadString('\n')
		if line == "" {
			break
		}
		if reNewEntry.MatchString(line) {
			add++
		} else if reDelEntry.MatchString(line) {
			del++
		}

		if err != nil {
			break
		}
	}

	_ = cmd.Wait()

	diffStat := ""
	if add != 0 {
		diffStat = fmt.Sprintf("%d new", add)
	}
	if del != 0 {
		if diffStat != "" {
			diffStat += ", "
		}
		diffStat += fmt.Sprintf("%d removed", del)
	}
	fmt.Printf("# Diff between %s and %s\n",
		filepath.Base(src), filepath.Base(dest))
	if diffStat == "" {
		fmt.Println("\tNothing changed.")
	}

	fmt.Println(diffStat)
	return true
}
