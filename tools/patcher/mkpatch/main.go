package main

import (
	"bufio"
	"bytes"
	"compress/zlib"
	"crypto/sha1"
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const B = 64

type op struct {
	kind     byte
	off, n   uint64
	data     []byte
}

func weak(b []byte) (uint32, uint32) {
	var a, s uint32
	for i, c := range b {
		a += uint32(c)
		s += uint32(len(b)-i) * uint32(c)
	}
	return a & 0xffff, s & 0xffff
}

func delta(old, nw []byte) []op {
	idx := map[uint32][]int{}
	for p := 0; p+B <= len(old); p += B {
		a, s := weak(old[p : p+B])
		k := a | s<<16
		if len(idx[k]) < 16 {
			idx[k] = append(idx[k], p)
		}
	}
	var ops []op
	lit := []byte{}
	flush := func() {
		if len(lit) > 0 {
			ops = append(ops, op{kind: 1, n: uint64(len(lit)), data: append([]byte(nil), lit...)})
			lit = lit[:0]
		}
	}
	i := 0
	var a, s uint32
	have := false
	for i < len(nw) {
		if i+B > len(nw) {
			lit = append(lit, nw[i:]...)
			break
		}
		if !have {
			a, s = weak(nw[i : i+B])
			have = true
		}
		k := (a & 0xffff) | (s&0xffff)<<16
		matched := false
		if cands, ok := idx[k]; ok {
			best, bestLen := -1, 0
			for _, p := range cands {
				if bytes.Equal(old[p:p+B], nw[i:i+B]) {
					l := B
					for p+l < len(old) && i+l < len(nw) && old[p+l] == nw[i+l] {
						l++
					}
					if l > bestLen {
						best, bestLen = p, l
					}
				}
			}
			if best >= 0 {
				// extend backwards into literal
				back := 0
				for back < len(lit) && best-back-1 >= 0 && old[best-back-1] == lit[len(lit)-1-back] {
					back++
				}
				lit = lit[:len(lit)-back]
				flush()
				ops = append(ops, op{kind: 0, off: uint64(best - back), n: uint64(bestLen + back)})
				i += bestLen
				have = false
				matched = true
			}
		}
		if !matched {
			lit = append(lit, nw[i])
			// roll
			if i+B < len(nw) {
				out, in := uint32(nw[i]), uint32(nw[i+B])
				a = (a - out + in) & 0xffff
				s = (s - uint32(B)*out + a) & 0xffff
			} else {
				have = false
			}
			i++
		}
	}
	flush()
	return ops
}

func main() {
	if len(os.Args) != 5 {
		fmt.Println("mkpatch <origRoot> <newRoot> <listfile> <out.bin>")
		os.Exit(1)
	}
	origRoot, newRoot := os.Args[1], os.Args[2]
	lf, _ := os.Open(os.Args[3])
	sc := bufio.NewScanner(lf)
	var paths []string
	for sc.Scan() {
		t := strings.TrimSpace(sc.Text())
		if t != "" {
			paths = append(paths, t)
		}
	}
	var buf bytes.Buffer
	buf.WriteString("TROYKO01")
	binary.Write(&buf, binary.LittleEndian, uint32(len(paths)))
	var tot, lits uint64
	for _, p := range paths {
		old, err := os.ReadFile(filepath.Join(origRoot, p))
		if err != nil {
			panic(err)
		}
		nw, err := os.ReadFile(filepath.Join(newRoot, p))
		if err != nil {
			panic(err)
		}
		ops := delta(old, nw)
		// verify
		var rb []byte
		for _, o := range ops {
			if o.kind == 0 {
				rb = append(rb, old[o.off:o.off+o.n]...)
			} else {
				rb = append(rb, o.data...)
				lits += o.n
			}
		}
		if !bytes.Equal(rb, nw) {
			panic("verify failed " + p)
		}
		tot += uint64(len(nw))
		binary.Write(&buf, binary.LittleEndian, uint16(len(p)))
		buf.WriteString(p)
		oh, nh := sha1.Sum(old), sha1.Sum(nw)
		binary.Write(&buf, binary.LittleEndian, uint64(len(old)))
		buf.Write(oh[:])
		binary.Write(&buf, binary.LittleEndian, uint64(len(nw)))
		buf.Write(nh[:])
		binary.Write(&buf, binary.LittleEndian, uint32(len(ops)))
		for _, o := range ops {
			buf.WriteByte(o.kind)
			if o.kind == 0 {
				binary.Write(&buf, binary.LittleEndian, o.off)
				binary.Write(&buf, binary.LittleEndian, o.n)
			} else {
				binary.Write(&buf, binary.LittleEndian, o.n)
				buf.Write(o.data)
			}
		}
	}
	f, _ := os.Create(os.Args[4])
	zw, _ := zlib.NewWriterLevel(f, zlib.BestCompression)
	zw.Write(buf.Bytes())
	zw.Close()
	f.Close()
	st, _ := os.Stat(os.Args[4])
	fmt.Printf("files %d, new bytes %d, literal bytes %d, patch %d\n", len(paths), tot, lits, st.Size())
}
