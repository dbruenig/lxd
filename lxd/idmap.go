package main

import (
	"bufio"
	"fmt"
	"os"
	"os/user"
	"path"
	"strconv"
	"strings"

	"github.com/lxc/lxd/fuidshift"
)

/*
 * We'll flesh this out to be lists of ranges
 * We will want a list of available ranges (all ranges
 * which lxd may use) and taken range (parts of the
 * available ranges which are already in use by containers)
 *
 * We also may want some way of deciding which containers may
 * or perhaps must not share ranges
 *
 * For now, we simply have a single range, shared by all
 * containers
 */
type Idmap struct {
	Uidmin, Uidrange uint
	Gidmin, Gidrange uint
}

const (
	minIDRange = 65536
)

func checkmap(fname string, username string) (uint, uint, error) {
	f, err := os.Open(fname)
	var min uint
	var idrange uint
	if err != nil {
		return 0, 0, err
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	min = 0
	idrange = 0
	for scanner.Scan() {
		/*
		 * /etc/sub{gu}id allow comments in the files, so ignore
		 * everything after a '#'
		 */
		s := strings.Split(scanner.Text(), "#")
		if len(s[0]) == 0 {
			continue
		}

		s = strings.Split(s[0], ":")
		if len(s) < 3 {
			return 0, 0, fmt.Errorf("unexpected values in %q: %q", fname, s)
		}
		if strings.EqualFold(s[0], username) {
			bigmin, err := strconv.ParseUint(s[1], 10, 32)
			if err != nil {
				continue
			}
			bigIdrange, err := strconv.ParseUint(s[2], 10, 32)
			if err != nil {
				continue
			}
			min = uint(bigmin)
			idrange = uint(bigIdrange)
			return min, idrange, nil
		}
	}

	return 0, 0, fmt.Errorf("User %q has no %ss.", username, path.Base(fname))
}

func NewIdmap() (*Idmap, error) {
	me, err := user.Current()
	if err != nil {
		return nil, err
	}

	m := new(Idmap)
	umin, urange, err := checkmap("/etc/subuid", me.Username)
	if err != nil {
		return nil, err
	}
	gmin, grange, err := checkmap("/etc/subgid", me.Username)
	if err != nil {
		return nil, err
	}

	if urange < minIDRange {
		return nil, fmt.Errorf("uidrange less than %d", minIDRange)
	}
	if grange < minIDRange {
		return nil, fmt.Errorf("gidrange less than %d", minIDRange)
	}

	m.Uidmin = umin
	m.Uidrange = urange
	m.Gidmin = gmin
	m.Gidrange = grange
	return m, nil
}

func (i *Idmap) ShiftRootfs(p string) error {
	set := fuidshift.IdmapSet{}
	uidstr := fmt.Sprintf("u:0:%d:%d", i.Uidmin, i.Uidrange)
	gidstr := fmt.Sprintf("g:0:%d:%d", i.Gidmin, i.Gidrange)
	set, err := set.Append(uidstr)
	if err != nil {
		return err
	}
	set, err = set.Append(gidstr)
	if err != nil {
		return err
	}
	return fuidshift.Uidshift(p, set, false)
}
