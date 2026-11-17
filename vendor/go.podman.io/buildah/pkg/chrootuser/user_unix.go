//go:build linux || freebsd

package chrootuser

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/user"
	"strconv"
	"strings"
	"sync"

	"go.podman.io/storage/pkg/reexec"
	"golang.org/x/sys/unix"
)

const (
	openChrootedCommand = "chrootuser-open"
)

func init() {
	reexec.Register(openChrootedCommand, openChrootedFileMain)
}

func openChrootedFileMain() {
	status := 0
	flag.Parse()
	if len(flag.Args()) < 1 {
		os.Exit(1)
	}
	// Our first parameter is the directory to chroot into.
	if err := unix.Chdir(flag.Arg(0)); err != nil {
		fmt.Fprintf(os.Stderr, "chdir(): %v", err)
		os.Exit(1)
	}
	if err := unix.Chroot(flag.Arg(0)); err != nil {
		fmt.Fprintf(os.Stderr, "chroot(): %v", err)
		os.Exit(1)
	}
	// Anything else is a file we want to dump out.
	for _, filename := range flag.Args()[1:] {
		f, err := os.Open(filename)
		if err != nil {
			fmt.Fprintf(os.Stderr, "open(%q): %v", filename, err)
			status = 1
			continue
		}
		_, err = io.Copy(os.Stdout, f)
		if err != nil {
			fmt.Fprintf(os.Stderr, "read(%q): %v", filename, err)
		}
		f.Close()
	}
	os.Exit(status)
}

func openChrootedFile(rootdir, filename string) (*exec.Cmd, io.ReadCloser, error) {
	// The child process expects a chroot and one or more filenames that
	// will be consulted relative to the chroot directory and concatenated
	// to its stdout.  Start it up.
	cmd := reexec.Command(openChrootedCommand, rootdir, filename)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, nil, err
	}
	err = cmd.Start()
	if err != nil {
		return nil, nil, err
	}
	// Hand back the child's stdout for reading, and the child to reap.
	return cmd, stdout, nil
}

var lookupUser, lookupGroup sync.Mutex

type lookupPasswdEntry struct {
	name string
	uid  uint64
	gid  uint64
	home string
}
type lookupGroupEntry struct {
	name string
	gid  uint64
	user string
}

func scanWithoutComments(rc *bufio.Scanner) (string, bool) {
	for {
		if !rc.Scan() {
			return "", false
		}
		line := rc.Text()
		if strings.HasPrefix(strings.TrimSpace(line), "#") {
			continue
		}
		return line, true
	}
}

// parseNextPasswd returns the next entry it can parse. Lines it cannot parse
// are skipped, so one bad line does not hide the entries after it. It returns
// nil at the end of the file and when the scanner failed, so callers need to
// check rc.Err() to tell those two apart.
func parseNextPasswd(rc *bufio.Scanner) *lookupPasswdEntry {
	for rc.Scan() {
		fields := strings.Split(rc.Text(), ":")
		if len(fields) != 7 {
			continue
		}
		uid, err := strconv.ParseUint(fields[2], 10, 32)
		if err != nil {
			continue
		}
		gid, err := strconv.ParseUint(fields[3], 10, 32)
		if err != nil {
			continue
		}
		return &lookupPasswdEntry{
			name: fields[0],
			uid:  uid,
			gid:  gid,
			home: fields[5],
		}
	}
	return nil
}

// parseNextGroup behaves like parseNextPasswd, see the comment there.
func parseNextGroup(rc *bufio.Scanner) *lookupGroupEntry {
	for {
		// On FreeBSD, /etc/group may contain comments:
		//   https://man.freebsd.org/cgi/man.cgi?query=group&sektion=5&format=html
		// We need to ignore those lines rather than trying to parse them.
		line, ok := scanWithoutComments(rc)
		if !ok {
			return nil
		}
		fields := strings.Split(line, ":")
		if len(fields) != 4 {
			continue
		}
		gid, err := strconv.ParseUint(fields[2], 10, 32)
		if err != nil {
			continue
		}
		return &lookupGroupEntry{
			name: fields[0],
			gid:  gid,
			user: fields[3],
		}
	}
}

func lookupUserInContainer(rootdir, username string) (uid uint64, gid uint64, err error) {
	cmd, f, err := openChrootedFile(rootdir, "/etc/passwd")
	if err != nil {
		return 0, 0, err
	}
	defer func() {
		_ = cmd.Wait()
	}()
	rc := bufio.NewScanner(f)
	defer f.Close()

	lookupUser.Lock()
	defer lookupUser.Unlock()

	pwd := parseNextPasswd(rc)
	for pwd != nil {
		if pwd.name != username {
			pwd = parseNextPasswd(rc)
			continue
		}
		return pwd.uid, pwd.gid, nil
	}

	if err := rc.Err(); err != nil {
		return 0, 0, fmt.Errorf("reading /etc/passwd: %w", err)
	}
	return 0, 0, user.UnknownUserError(fmt.Sprintf("error looking up user %q", username))
}

func lookupGroupForUIDInContainer(rootdir string, userid uint64) (username string, gid uint64, err error) {
	cmd, f, err := openChrootedFile(rootdir, "/etc/passwd")
	if err != nil {
		return "", 0, err
	}
	defer func() {
		_ = cmd.Wait()
	}()
	rc := bufio.NewScanner(f)
	defer f.Close()

	lookupUser.Lock()
	defer lookupUser.Unlock()

	pwd := parseNextPasswd(rc)
	for pwd != nil {
		if pwd.uid != userid {
			pwd = parseNextPasswd(rc)
			continue
		}
		return pwd.name, pwd.gid, nil
	}

	if err := rc.Err(); err != nil {
		return "", 0, fmt.Errorf("reading /etc/passwd: %w", err)
	}
	return "", 0, ErrNoSuchUser
}

func lookupAdditionalGroupsForUIDInContainer(rootdir string, userid uint64) (gid []uint32, err error) {
	// Get the username associated with userid
	username, _, err := lookupGroupForUIDInContainer(rootdir, userid)
	if err != nil {
		return nil, err
	}

	cmd, f, err := openChrootedFile(rootdir, "/etc/group")
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = cmd.Wait()
	}()
	rc := bufio.NewScanner(f)
	defer f.Close()

	lookupGroup.Lock()
	defer lookupGroup.Unlock()

	grp := parseNextGroup(rc)
	for grp != nil {
		if strings.Contains(grp.user, username) {
			gid = append(gid, uint32(grp.gid))
		}
		grp = parseNextGroup(rc)
	}
	if err := rc.Err(); err != nil {
		return nil, fmt.Errorf("reading /etc/group: %w", err)
	}
	return gid, nil
}

func lookupGroupInContainer(rootdir, groupname string) (gid uint64, err error) {
	cmd, f, err := openChrootedFile(rootdir, "/etc/group")
	if err != nil {
		return 0, err
	}
	defer func() {
		_ = cmd.Wait()
	}()
	rc := bufio.NewScanner(f)
	defer f.Close()

	lookupGroup.Lock()
	defer lookupGroup.Unlock()

	grp := parseNextGroup(rc)
	for grp != nil {
		if grp.name != groupname {
			grp = parseNextGroup(rc)
			continue
		}
		return grp.gid, nil
	}

	if err := rc.Err(); err != nil {
		return 0, fmt.Errorf("reading /etc/group: %w", err)
	}
	return 0, user.UnknownGroupError(fmt.Sprintf("error looking up group %q", groupname))
}

func lookupUIDInContainer(rootdir string, uid uint64) (string, uint64, error) {
	cmd, f, err := openChrootedFile(rootdir, "/etc/passwd")
	if err != nil {
		return "", 0, err
	}
	defer func() {
		_ = cmd.Wait()
	}()
	rc := bufio.NewScanner(f)
	defer f.Close()

	lookupUser.Lock()
	defer lookupUser.Unlock()

	pwd := parseNextPasswd(rc)
	for pwd != nil {
		if pwd.uid != uid {
			pwd = parseNextPasswd(rc)
			continue
		}
		return pwd.name, pwd.gid, nil
	}

	if err := rc.Err(); err != nil {
		return "", 0, fmt.Errorf("reading /etc/passwd: %w", err)
	}
	return "", 0, user.UnknownUserError(fmt.Sprintf("error looking up uid %d", uid))
}

func lookupHomedirInContainer(rootdir string, uid uint64) (string, error) {
	cmd, f, err := openChrootedFile(rootdir, "/etc/passwd")
	if err != nil {
		return "", err
	}
	defer func() {
		_ = cmd.Wait()
	}()
	rc := bufio.NewScanner(f)
	defer f.Close()

	lookupUser.Lock()
	defer lookupUser.Unlock()

	pwd := parseNextPasswd(rc)
	for pwd != nil {
		if pwd.uid != uid {
			pwd = parseNextPasswd(rc)
			continue
		}
		return pwd.home, nil
	}

	if err := rc.Err(); err != nil {
		return "", fmt.Errorf("reading /etc/passwd: %w", err)
	}
	return "", user.UnknownUserError(fmt.Sprintf("error looking up homedir for uid %d", uid))
}
