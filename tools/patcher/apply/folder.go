package main

import (
	"crypto/sha1"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const backupDir = "TroyKOR_backup"

// resolve finds rel (slash separated, any case) under root, matching each
// component case-insensitively. It returns the real path on disk.
func resolve1(root, rel string) (string, error) {
	cur := root
	for _, part := range strings.Split(rel, "/") {
		if part == "" {
			continue
		}
		p := filepath.Join(cur, part)
		if _, err := os.Stat(p); err == nil {
			cur = p
			continue
		}
		ents, err := os.ReadDir(cur)
		if err != nil {
			return "", err
		}
		found := ""
		for _, e := range ents {
			if strings.EqualFold(e.Name(), part) {
				found = e.Name()
				break
			}
		}
		if found == "" {
			return "", fmt.Errorf("파일을 찾을 수 없습니다: %s", rel)
		}
		cur = filepath.Join(cur, found)
	}
	return cur, nil
}

// findGameRoot accepts the folder that contains PS3_GAME, or PS3_GAME itself.
func findGameRoot(dir string) (string, error) {
	if p, err := resolve1(dir, "PS3_GAME/PARAM.SFO"); err == nil {
		_ = p
		return dir, nil
	}
	if _, err := resolve1(dir, "PARAM.SFO"); err == nil && strings.EqualFold(filepath.Base(dir), "PS3_GAME") {
		return filepath.Dir(dir), nil
	}
	return "", fmt.Errorf("게임 폴더가 아닙니다. PS3_GAME 폴더가 들어 있는 폴더를 넣어 주세요.\n(%s)", dir)
}

func writeAtomic(path string, data []byte) error {
	tmp := path + ".troykor_tmp"
	if err := os.WriteFile(tmp, data, 0644); err != nil {
		os.Remove(tmp)
		return err
	}
	os.Chmod(path, 0644) // read-only files would block the rename on Windows
	if err := os.Rename(tmp, path); err != nil {
		os.Remove(tmp)
		return err
	}
	return nil
}

func runFolder(files []pfile, dir string) error {
	root, err := findGameRoot(dir)
	if err != nil {
		return err
	}
	fmt.Printf("\n게임 폴더: %s\n", root)

	sfoPath, err := resolve1(root, "PS3_GAME/PARAM.SFO")
	if err != nil {
		return err
	}
	sfo, err := os.ReadFile(sfoPath)
	if err != nil {
		return err
	}
	if t := paramTitle(sfo); t != titleID {
		if t == "" {
			return fmt.Errorf("PARAM.SFO를 읽을 수 없습니다")
		}
		return fmt.Errorf("이 패치는 %s 전용입니다. (이 폴더: %s)", titleID, t)
	}

	files, err = resolve(func(rel string) ([]byte, error) {
		p, err := resolve1(root, rel)
		if err != nil {
			return nil, err
		}
		return readKnown(rel, p)
	})
	if err != nil {
		return err
	}
	type job struct {
		pf   pfile
		path string
		old  []byte
	}
	var jobs []job
	patched := 0
	fixedTail := 0
	for _, pf := range files {
		p, err := resolve1(root, pf.path)
		if err != nil {
			return err
		}
		b, err := readKnown(pf.path, p)
		if err != nil {
			return err
		}
		h := sha1.Sum(b)
		if h == pf.newSHA && uint64(len(b)) == pf.newSize {
			if st, err := os.Stat(p); err == nil && st.Size() != int64(len(b)) {
				// patched content with leftover bytes: cut them off
				if err := writeAtomic(p, b); err != nil {
					return fmt.Errorf("쓰기 실패: %s (%v)", pf.path, err)
				}
				fixedTail++
			}
			patched++
			continue
		}
		if h != pf.origSHA {
			return mismatchMsg(pf.path, "폴더")
		}
		jobs = append(jobs, job{pf, p, b})
	}

	bdir := filepath.Join(root, backupDir)
	if fixedTail > 0 {
		fmt.Printf("\n뒤에 원본 찌꺼기가 붙은 파일 %d개를 바로잡았습니다.\n", fixedTail)
	}
	if len(jobs) == 0 {
		fmt.Println("\n이미 한글패치가 적용된 폴더입니다.")
		if _, err := os.Stat(bdir); err == nil && edition != "update" {
			if a := ask("원본으로 되돌리시겠습니까? (y/N): "); strings.EqualFold(a, "y") {
				return restoreFolder(files, root, bdir)
			}
		}
		return nil
	}

	if edition != "update" {
		fmt.Printf("\n원본 파일을 %s 폴더에 백업합니다.\n", backupDir)
	}
	for _, j := range jobs {
		if edition == "update" || j.pf.origSHA != fullOrig(j.pf.path) {
			continue // 백업에는 게임 원본 파일만 둡니다
		}
		bp := filepath.Join(bdir, filepath.FromSlash(j.pf.path))
		if _, err := os.Stat(bp); err == nil {
			continue
		}
		if err := os.MkdirAll(filepath.Dir(bp), 0755); err != nil {
			return fmt.Errorf("백업 폴더를 만들 수 없습니다: %v", err)
		}
		if err := os.WriteFile(bp, j.old, 0644); err != nil {
			return fmt.Errorf("백업 실패: %v", err)
		}
	}

	fmt.Printf("\n파일 %d개 적용 중...\n", len(jobs))
	for i, j := range jobs {
		var nb []byte
		for _, o := range j.pf.ops {
			if o.kind == 0 {
				nb = append(nb, j.old[o.off:o.off+o.n]...)
			} else {
				nb = append(nb, o.data...)
			}
		}
		if sha1.Sum(nb) != j.pf.newSHA {
			return fmt.Errorf("패치 결과 검증 실패: %s", j.pf.path)
		}
		if err := writeAtomic(j.path, nb); err != nil {
			return fmt.Errorf("쓰기 실패: %s (%v)\n게임이나 다른 프로그램이 파일을 쓰고 있지 않은지 확인하세요.", j.pf.path, err)
		}
		fmt.Printf("\r  %d / %d", i+1, len(jobs))
	}
	fmt.Println()

	for _, pf := range files {
		p, _ := resolve1(root, pf.path)
		b, err := os.ReadFile(p)
		if err != nil || sha1.Sum(b) != pf.newSHA {
			return fmt.Errorf("적용 후 검증 실패: %s", pf.path)
		}
	}
	fmt.Println("\n완료! 한글패치가 적용되었습니다.")
	if _, err := os.Stat(bdir); err == nil {
		fmt.Printf("원본 파일은 %s 에 있습니다.\n", bdir)
		if edition != "update" {
			fmt.Println("(이 폴더를 다시 패처에 넣으면 원본으로 되돌릴 수 있습니다)")
		}
	}
	return nil
}

func restoreFolder(files []pfile, root, bdir string) error {
	n := 0
	for _, pf := range files {
		bp := filepath.Join(bdir, filepath.FromSlash(pf.path))
		b, err := os.ReadFile(bp)
		if err != nil {
			continue // file was already original when patched
		}
		if sha1.Sum(b) != fullOrig(pf.path) {
			return fmt.Errorf("백업 파일이 원본과 다릅니다: %s", pf.path)
		}
		p, err := resolve1(root, pf.path)
		if err != nil {
			return err
		}
		if err := writeAtomic(p, b); err != nil {
			return err
		}
		n++
	}
	left := 0
	for _, pf := range files {
		p, _ := resolve1(root, pf.path)
		b, err := os.ReadFile(p)
		if err != nil || sha1.Sum(b) != fullOrig(pf.path) {
			left++
		}
	}
	fmt.Printf("\n완료! 파일 %d개를 원본으로 되돌렸습니다.\n", n)
	if left > 0 {
		fmt.Printf("※ 백업이 없어 되돌리지 못한 파일이 %d개 있습니다.\n", left)
		fmt.Println("  (패처가 아닌 다른 방법으로 바뀐 파일입니다. 원본 ISO에서 다시 꺼내 주세요)")
		return nil
	}
	os.RemoveAll(bdir)
	return nil
}

// fullOrig returns the SHA-1 of the untouched game file for path.
func fullOrig(path string) [20]byte {
	for _, pf := range fullSet {
		if pf.path == path {
			return pf.origSHA
		}
	}
	return [20]byte{}
}

// readKnown reads a game file. Folders extracted from an ISO patched by
// v1.0~v1.0.1 with a UDF-reading tool can contain patched files followed by
// leftover bytes of the original (the UDF size was not updated). If a prefix
// of the file matches a known version exactly, that prefix is used.
func readKnown(rel, p string) ([]byte, error) {
	b, err := os.ReadFile(p)
	if err != nil {
		return nil, err
	}
	h := sha1.Sum(b)
	type cand struct {
		n   uint64
		sha [20]byte
	}
	var cs []cand
	for _, set := range sets {
		for _, pf := range set {
			if pf.path != rel {
				continue
			}
			if h == pf.origSHA || h == pf.newSHA {
				return b, nil
			}
			cs = append(cs, cand{pf.origSize, pf.origSHA}, cand{pf.newSize, pf.newSHA})
		}
	}
	for _, c := range cs {
		if c.n > 0 && c.n < uint64(len(b)) && sha1.Sum(b[:c.n]) == c.sha {
			return b[:c.n], nil
		}
	}
	return b, nil
}
