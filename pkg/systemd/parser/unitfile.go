package parser

import (
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"os/user"
	"path"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"unicode"
)

// This code (UnitFile) support reading well formed files in the same
// format as the systmed unit files. It can also regenerate the file
// essentially identically, including comments and group/key order.
// The only thing that is modified is that multiple instances of one
// group are merged.

// There is also support for reading and modifying keys while the
// UnitFile is in memory, including support for systemd-like slitting
// of argument lines and escaping/unescaping of text.

type unitLine struct {
	key       string
	value     string
	isComment bool
}

type unitGroup struct {
	name     string
	comments []*unitLine // Comments before the groupname
	lines    []*unitLine
}

type UnitFile struct {
	groups      []*unitGroup
	groupByName map[string]*unitGroup

	Filename string
	Path     string
}

type UnitFileParser struct {
	file *UnitFile

	currentGroup    *unitGroup
	pendingComments []*unitLine
}

func newUnitLine(key string, value string, isComment bool) *unitLine {
	l := &unitLine{
		key:       key,
		value:     value,
		isComment: isComment,
	}
	return l
}

func (l *unitLine) set(value string) {
	l.value = value
}

func (l *unitLine) dup() *unitLine {
	return newUnitLine(l.key, l.value, l.isComment)
}

func (l *unitLine) isKey(key string) bool {
	return !l.isComment &&
		l.key == key
}

func (l *unitLine) isEmpty() bool {
	return len(l.value) == 0
}

func newUnitGroup(name string) *unitGroup {
	g := &unitGroup{
		name:     name,
		comments: make([]*unitLine, 0),
		lines:    make([]*unitLine, 0),
	}
	return g
}

func (g *unitGroup) addLine(line *unitLine) {
	g.lines = append(g.lines, line)
}

func (g *unitGroup) prependLine(line *unitLine) {
	n := make([]*unitLine, 0, 1+len(g.lines))
	n = append(n, line)
	g.lines = append(n, g.lines...)
}

func (g *unitGroup) addComment(line *unitLine) {
	g.comments = append(g.comments, line)
}

func (g *unitGroup) prependComment(line *unitLine) {
	n := make([]*unitLine, 0, 1+len(g.comments))
	n = append(n, line)
	g.comments = append(n, g.comments...)
}

func (g *unitGroup) add(key string, value string) {
	g.addLine(newUnitLine(key, value, false))
}

func (g *unitGroup) findLast(key string) *unitLine {
	for _, l := range slices.Backward(g.lines) {
		if l.isKey(key) {
			return l
		}
	}

	return nil
}

func (g *unitGroup) set(key string, value string) {
	line := g.findLast(key)
	if line != nil {
		line.set(value)
	} else {
		g.add(key, value)
	}
}

func (g *unitGroup) unset(key string) {
	newlines := make([]*unitLine, 0, len(g.lines))

	for _, line := range g.lines {
		if !line.isKey(key) {
			newlines = append(newlines, line)
		}
	}
	g.lines = newlines
}

func (g *unitGroup) merge(source *unitGroup) {
	for _, l := range source.comments {
		g.comments = append(g.comments, l.dup())
	}
	for _, l := range source.lines {
		g.lines = append(g.lines, l.dup())
	}
}

// Create an empty unit file, with no filename or path
func NewUnitFile() *UnitFile {
	f := &UnitFile{
		groups:      make([]*unitGroup, 0),
		groupByName: make(map[string]*unitGroup),
	}

	return f
}

// Load a unit file from disk, remembering the path and filename
func ParseUnitFile(pathName string) (*UnitFile, error) {
	data, e := os.ReadFile(pathName)
	if e != nil {
		return nil, e
	}

	f := NewUnitFile()
	f.Path = pathName
	f.Filename = path.Base(pathName)

	if e := f.Parse(string(data)); e != nil {
		return nil, e
	}

	return f, nil
}

func (f *UnitFile) ensureGroup(groupName string) *unitGroup {
	if g, ok := f.groupByName[groupName]; ok {
		return g
	}

	g := newUnitGroup(groupName)
	f.groups = append(f.groups, g)
	f.groupByName[groupName] = g

	return g
}

func (f *UnitFile) Merge(source *UnitFile) {
	for _, srcGroup := range source.groups {
		group := f.ensureGroup(srcGroup.name)
		group.merge(srcGroup)
	}
}

// Create a copy of the unit file, copies filename but not path
func (f *UnitFile) Dup() *UnitFile {
	copy := NewUnitFile()

	copy.Merge(f)
	copy.Filename = f.Filename
	return copy
}

func lineIsComment(line string) bool {
	return len(line) == 0 || line[0] == '#' || line[0] == ';'
}

func lineIsGroup(line string) bool {
	if len(line) == 0 {
		return false
	}

	if line[0] != '[' {
		return false
	}

	end := strings.Index(line, "]")
	if end == -1 {
		return false
	}

	// silently accept whitespace after the ]
	for i := end + 1; i < len(line); i++ {
		if line[i] != ' ' && line[i] != '\t' {
			return false
		}
	}

	return true
}

func lineIsKeyValuePair(line string) bool {
	if len(line) == 0 {
		return false
	}

	p := strings.IndexByte(line, '=')
	if p == -1 {
		return false
	}

	// Key must be non-empty
	if p == 0 {
		return false
	}

	return true
}

func groupNameIsValid(name string) bool {
	if len(name) == 0 {
		return false
	}

	for _, c := range name {
		if c == ']' || c == '[' || unicode.IsControl(c) {
			return false
		}
	}

	return true
}

func keyNameIsValid(name string) bool {
	if len(name) == 0 {
		return false
	}

	for _, c := range name {
		if c == '=' {
			return false
		}
	}

	// No leading/trailing space
	if name[0] == ' ' || name[len(name)-1] == ' ' {
		return false
	}

	return true
}

func (p *UnitFileParser) parseComment(line string) error {
	l := newUnitLine("", line, true)
	p.pendingComments = append(p.pendingComments, l)
	return nil
}

func (p *UnitFileParser) parseGroup(line string) error {
	end := strings.Index(line, "]")

	groupName := line[1:end]

	if !groupNameIsValid(groupName) {
		return fmt.Errorf("invalid group name: %s", groupName)
	}

	p.currentGroup = p.file.ensureGroup(groupName)

	if p.pendingComments != nil {
		firstComment := p.pendingComments[0]

		// Remove one newline between groups, which is re-added on
		// printing, see unitGroup.Write()
		if firstComment.isEmpty() {
			p.pendingComments = p.pendingComments[1:]
		}

		p.flushPendingComments(true)
	}

	return nil
}

func (p *UnitFileParser) parseKeyValuePair(line string) error {
	if p.currentGroup == nil {
		return fmt.Errorf("key file does not start with a group")
	}

	keyEnd := strings.Index(line, "=")
	valueStart := keyEnd + 1

	// Pull the key name from the line (chomping trailing whitespace)
	for keyEnd > 0 && unicode.IsSpace(rune(line[keyEnd-1])) {
		keyEnd--
	}
	key := line[:keyEnd]
	if !keyNameIsValid(key) {
		return fmt.Errorf("invalid key name: %s", key)
	}

	// Pull the value from the line (chugging leading whitespace)

	for valueStart < len(line) && unicode.IsSpace(rune(line[valueStart])) {
		valueStart++
	}

	value := line[valueStart:]

	p.flushPendingComments(false)

	p.currentGroup.add(key, value)

	return nil
}

func (p *UnitFileParser) parseLine(line string, lineNr int) error {
	switch {
	case lineIsComment(line):
		return p.parseComment(line)
	case lineIsGroup(line):
		return p.parseGroup(line)
	case lineIsKeyValuePair(line):
		return p.parseKeyValuePair(line)
	default:
		return fmt.Errorf("file contains line %d: “%s” which is not a key-value pair, group, or comment", lineNr, line)
	}
}

func (p *UnitFileParser) flushPendingComments(toComment bool) {
	pending := p.pendingComments
	if pending == nil {
		return
	}
	p.pendingComments = nil

	for _, pendingLine := range pending {
		if toComment {
			p.currentGroup.addComment(pendingLine)
		} else {
			p.currentGroup.addLine(pendingLine)
		}
	}
}

// Parse an already loaded unit file (in the form of a string)
func (f *UnitFile) Parse(data string) error {
	p := &UnitFileParser{
		file: f,
	}

	lines := strings.Split(strings.TrimSuffix(data, "\n"), "\n")
	remaining := ""

	for lineNr, line := range lines {
		line = strings.TrimSpace(line)
		if lineIsComment(line) {
			// ignore the comment is inside a continuation line.
			if remaining != "" {
				continue
			}
		} else {
			if strings.HasSuffix(line, "\\") {
				line = line[:len(line)-1]
				if lineNr != len(lines)-1 {
					remaining += line
					continue
				}
			}
			// check whether the line is a continuation of the previous line
			if remaining != "" {
				line = remaining + line
				remaining = ""
			}
		}
		if err := p.parseLine(line, lineNr+1); err != nil {
			return err
		}
	}

	if p.currentGroup == nil {
		// For files without groups, add an empty group name used only for initial comments
		p.currentGroup = p.file.ensureGroup("")
	}
	p.flushPendingComments(false)

	return nil
}

func (l *unitLine) write(w io.Writer) error {
	if l.isComment {
		if _, err := fmt.Fprintf(w, "%s\n", l.value); err != nil {
			return err
		}
	} else {
		if _, err := fmt.Fprintf(w, "%s=%s\n", l.key, l.value); err != nil {
			return err
		}
	}

	return nil
}

func (g *unitGroup) write(w io.Writer) error {
	for _, c := range g.comments {
		if err := c.write(w); err != nil {
			return err
		}
	}

	if g.name == "" {
		// Empty name groups are not valid, but used internally to handle comments in empty files
		return nil
	}

	if _, err := fmt.Fprintf(w, "[%s]\n", g.name); err != nil {
		return err
	}

	for _, l := range g.lines {
		if err := l.write(w); err != nil {
			return err
		}
	}

	return nil
}

// Convert a UnitFile back to data, writing to the io.Writer w
func (f *UnitFile) Write(w io.Writer) error {
	for i, g := range f.groups {
		// We always add a newline between groups, and strip one if it exists during
		// parsing. This looks nicer, and avoids issues of duplicate newlines when
		// merging groups or missing ones when creating new groups
		if i != 0 {
			if _, err := io.WriteString(w, "\n"); err != nil {
				return err
			}
		}

		if err := g.write(w); err != nil {
			return err
		}
	}

	return nil
}

// Convert a UnitFile back to data, as a string
func (f *UnitFile) ToString() (string, error) {
	var str strings.Builder
	if err := f.Write(&str); err != nil {
		return "", err
	}
	return str.String(), nil
}

func applyLineContinuation(raw string) string {
	if !strings.Contains(raw, "\\\n") {
		return raw
	}

	var str strings.Builder

	for len(raw) > 0 {
		if first, rest, found := strings.Cut(raw, "\\\n"); found {
			str.WriteString(first)
			raw = rest
		} else {
			str.WriteString(raw)
			raw = ""
		}
	}

	return str.String()
}

func (f *UnitFile) HasGroup(groupName string) bool {
	_, ok := f.groupByName[groupName]
	return ok
}

func (f *UnitFile) RemoveGroup(groupName string) {
	g, ok := f.groupByName[groupName]
	if ok {
		delete(f.groupByName, groupName)

		newgroups := make([]*unitGroup, 0, len(f.groups))
		for _, oldgroup := range f.groups {
			if oldgroup != g {
				newgroups = append(newgroups, oldgroup)
			}
		}
		f.groups = newgroups
	}
}

func (f *UnitFile) RenameGroup(groupName string, newName string) {
	group, okOld := f.groupByName[groupName]
	if !okOld {
		return
	}

	newGroup, okNew := f.groupByName[newName]
	if !okNew {
		// New group doesn't exist, just rename in-place
		delete(f.groupByName, groupName)
		group.name = newName
		f.groupByName[newName] = group
	} else if group != newGroup {
		/* merge to existing group and delete old */
		newGroup.merge(group)
		f.RemoveGroup(groupName)
	}
}

func (f *UnitFile) ListGroups() []string {
	groups := make([]string, len(f.groups))
	for i, group := range f.groups {
		groups[i] = group.name
	}
	return groups
}

func (f *UnitFile) ListKeys(groupName string) []string {
	g, ok := f.groupByName[groupName]
	if !ok {
		return make([]string, 0)
	}

	hash := make(map[string]struct{})
	keys := make([]string, 0, len(g.lines))
	for _, line := range g.lines {
		if !line.isComment {
			if _, ok := hash[line.key]; !ok {
				keys = append(keys, line.key)
				hash[line.key] = struct{}{}
			}
		}
	}

	return keys
}

// Look up the last instance of the named key in the group (if any)
// The result can have trailing whitespace, and Raw means it can
// contain line continuations (\ at end of line)
func (f *UnitFile) LookupLastRaw(groupName string, key string) (string, bool) {
	g, ok := f.groupByName[groupName]
	if !ok {
		return "", false
	}

	line := g.findLast(key)
	if line == nil {
		return "", false
	}

	return line.value, true
}

func (f *UnitFile) HasKey(groupName string, key string) bool {
	_, ok := f.LookupLastRaw(groupName, key)
	return ok
}

// Look up the last instance of the named key in the group (if any)
// The result can have trailing whitespace, but line continuations are applied
func (f *UnitFile) LookupLast(groupName string, key string) (string, bool) {
	raw, ok := f.LookupLastRaw(groupName, key)
	if !ok {
		return "", false
	}

	return applyLineContinuation(raw), true
}

// Look up the last instance of the named key in the group (if any)
// The result have no trailing whitespace and line continuations are applied.
// Surrounding matched double-quote pairs are stripped, but unmatched quotes
// are preserved to avoid mangling embedded shell syntax.
func (f *UnitFile) Lookup(groupName string, key string) (string, bool) {
	v, ok := f.LookupLast(groupName, key)
	if !ok {
		return "", false
	}

	v = strings.TrimRightFunc(v, unicode.IsSpace)
	if len(v) >= 2 && v[0] == '"' && v[len(v)-1] == '"' {
		v = v[1 : len(v)-1]
	}
	return v, true
}

// Lookup the last instance of a key and convert the value to a bool
func (f *UnitFile) LookupBoolean(groupName string, key string) (bool, bool) {
	v, ok := f.Lookup(groupName, key)
	if !ok {
		return false, false
	}

	return strings.EqualFold(v, "1") ||
		strings.EqualFold(v, "yes") ||
		strings.EqualFold(v, "true") ||
		strings.EqualFold(v, "on"), true
}

// Lookup the last instance of a key and convert the value to a bool
func (f *UnitFile) LookupBooleanWithDefault(groupName string, key string, defaultValue bool) bool {
	v, ok := f.LookupBoolean(groupName, key)
	if !ok {
		return defaultValue
	}

	return v
}

/* Mimics strol, which is what systemd uses */
func convertNumber(v string) (int64, error) {
	var err error
	var intVal int64

	mult := int64(1)

	if strings.HasPrefix(v, "+") {
		v = v[1:]
	} else if strings.HasPrefix(v, "-") {
		v = v[1:]
		mult = int64(-1)
	}

	switch {
	case strings.HasPrefix(v, "0x") || strings.HasPrefix(v, "0X"):
		intVal, err = strconv.ParseInt(v[2:], 16, 64)
	case strings.HasPrefix(v, "0"):
		intVal, err = strconv.ParseInt(v, 8, 64)
	default:
		intVal, err = strconv.ParseInt(v, 10, 64)
	}

	return intVal * mult, err
}

// Lookup the last instance of a key and convert the value to an int64
func (f *UnitFile) LookupInt(groupName string, key string, defaultValue int64) int64 {
	v, ok := f.Lookup(groupName, key)
	if !ok {
		return defaultValue
	}

	intVal, err := convertNumber(v)
	if err != nil {
		return defaultValue
	}

	return intVal
}

// Lookup the last instance of a key and convert the value to an uint32
func (f *UnitFile) LookupUint32(groupName string, key string, defaultValue uint32) uint32 {
	v := f.LookupInt(groupName, key, int64(defaultValue))
	if v < 0 || v > math.MaxUint32 {
		return defaultValue
	}
	return uint32(v)
}

// Lookup the last instance of a key and convert a uid or a user name to an uint32 uid
func (f *UnitFile) LookupUID(groupName string, key string, defaultValue uint32) (uint32, error) {
	v, ok := f.Lookup(groupName, key)
	if !ok {
		if defaultValue == math.MaxUint32 {
			return 0, fmt.Errorf("no key %s", key)
		}
		return defaultValue, nil
	}

	intVal, err := convertNumber(v)
	if err == nil {
		/* On linux, uids are uint32 values, that can't be (uint32)-1 (== MAXUINT32)*/
		if intVal < 0 || intVal >= math.MaxUint32 {
			return 0, fmt.Errorf("invalid numerical uid '%s'", v)
		}

		return uint32(intVal), nil
	}

	user, err := user.Lookup(v)
	if err != nil {
		return 0, err
	}

	intVal, err = strconv.ParseInt(user.Uid, 10, 64)
	if err != nil {
		return 0, err
	}

	return uint32(intVal), nil
}

// Lookup the last instance of a key and convert a uid or a group name to an uint32 gid
func (f *UnitFile) LookupGID(groupName string, key string, defaultValue uint32) (uint32, error) {
	v, ok := f.Lookup(groupName, key)
	if !ok {
		if defaultValue == math.MaxUint32 {
			return 0, fmt.Errorf("no key %s", key)
		}
		return defaultValue, nil
	}

	intVal, err := convertNumber(v)
	if err == nil {
		/* On linux, uids are uint32 values, that can't be (uint32)-1 (== MAXUINT32)*/
		if intVal < 0 || intVal >= math.MaxUint32 {
			return 0, fmt.Errorf("invalid numerical uid '%s'", v)
		}

		return uint32(intVal), nil
	}

	group, err := user.LookupGroup(v)
	if err != nil {
		return 0, err
	}

	intVal, err = strconv.ParseInt(group.Gid, 10, 64)
	if err != nil {
		return 0, err
	}

	return uint32(intVal), nil
}

// Look up every instance of the named key in the group
// The result can have trailing whitespace, and Raw means it can
// contain line continuations (\ at end of line)
func (f *UnitFile) LookupAllRaw(groupName string, key string) []string {
	g, ok := f.groupByName[groupName]
	if !ok {
		return make([]string, 0)
	}

	values := make([]string, 0)

	for _, line := range g.lines {
		if line.isKey(key) {
			if len(line.value) == 0 {
				// Empty value clears all before
				values = make([]string, 0)
			} else {
				values = append(values, line.value)
			}
		}
	}

	return values
}

// Look up every instance of the named key in the group
// The result can have trailing whitespace, but line continuations are applied
func (f *UnitFile) LookupAll(groupName string, key string) []string {
	values := f.LookupAllRaw(groupName, key)
	for i, raw := range values {
		values[i] = applyLineContinuation(raw)
	}
	return values
}

// Look up every instance of the named key in the group, and for each, split space
// separated words (including handling quoted words) and combine them all into
// one array of words. The split code is compatible with the systemd config_parse_strv().
// This is typically used by systemd keys like "RequiredBy" and "Aliases".
func (f *UnitFile) LookupAllStrv(groupName string, key string) []string {
	res := make([]string, 0)
	values := f.LookupAll(groupName, key)
	for _, value := range values {
		res, _ = splitStringAppend(res, value, WhitespaceSeparators, SplitRetainEscape|SplitUnquote)
	}
	return res
}

// Look up every instance of the named key in the group, and for each, split space
// separated words (including handling quoted words) and combine them all into
// one array of words. The split code is exec-like, and both unquotes and applied
// c-style c escapes.
func (f *UnitFile) LookupAllArgs(groupName string, key string) []string {
	res := make([]string, 0)
	argsv := f.LookupAll(groupName, key)
	for _, argsS := range argsv {
		args, err := splitString(argsS, WhitespaceSeparators, SplitRelax|SplitUnquote|SplitCUnescape)
		if err == nil {
			res = append(res, args...)
		}
	}
	return res
}

// Look up last instance of the named key in the group, and split
// space separated words (including handling quoted words) into one
// array of words. The split code is exec-like, and both unquotes and
// applied c-style c escapes.  This is typically used for keys like
// ExecStart
func (f *UnitFile) LookupLastArgs(groupName string, key string) ([]string, bool, error) {
	execKey, ok := f.LookupLast(groupName, key)
	if !ok {
		return nil, false, nil
	}
	execArgs, err := splitString(execKey, WhitespaceSeparators, SplitRelax|SplitUnquote|SplitCUnescape)
	if err != nil {
		return nil, false, err
	}
	return execArgs, true, nil
}

// Look up 'Environment' style key-value keys
func (f *UnitFile) LookupAllKeyVal(groupName string, key string) (map[string]*string, error) {
	var warnings error
	res := make(map[string]*string)
	allKeyvals := f.LookupAll(groupName, key)
	for _, keyvals := range allKeyvals {
		assigns, err := splitString(keyvals, WhitespaceSeparators, SplitRelax|SplitUnquote|SplitCUnescape)
		if err != nil {
			warnings = errors.Join(warnings, err)
			continue
		}
		for _, assign := range assigns {
			key, value, found := strings.Cut(assign, "=")
			if found {
				res[key] = &value
			} else {
				res[key] = nil
			}
		}
	}
	return res, warnings
}

func (f *UnitFile) Set(groupName string, key string, value string) {
	group := f.ensureGroup(groupName)
	group.set(key, value)
}

func (f *UnitFile) Setv(groupName string, keyvals ...string) {
	group := f.ensureGroup(groupName)
	for i := 0; i+1 < len(keyvals); i += 2 {
		group.set(keyvals[i], keyvals[i+1])
	}
}

func (f *UnitFile) Add(groupName string, key string, value string) {
	group := f.ensureGroup(groupName)
	group.add(key, value)
}

func (f *UnitFile) AddEscaped(groupName string, key string, value string) {
	if wordNeedEscape(value) {
		var escaped strings.Builder
		appendEscapeWord(&escaped, value)
		value = escaped.String()
	}
	f.Add(groupName, key, value)
}

func (f *UnitFile) AddCmdline(groupName string, key string, args []string) {
	f.Add(groupName, key, escapeWords(args))
}

func (f *UnitFile) Unset(groupName string, key string) {
	group, ok := f.groupByName[groupName]
	if ok {
		group.unset(key)
	}
}

// Empty group name == first group
func (f *UnitFile) AddComment(groupName string, comments ...string) {
	var group *unitGroup
	if groupName == "" && len(f.groups) > 0 {
		group = f.groups[0]
	} else {
		// Uses magic "" for first comment-only group if no other groups
		group = f.ensureGroup(groupName)
	}

	for _, comment := range comments {
		group.addComment(newUnitLine("", "# "+comment, true))
	}
}

func (f *UnitFile) PrependComment(groupName string, comments ...string) {
	var group *unitGroup
	if groupName == "" && len(f.groups) > 0 {
		group = f.groups[0]
	} else {
		// Uses magic "" for first comment-only group if no other groups
		group = f.ensureGroup(groupName)
	}
	// Prepend in reverse order to keep argument order
	for _, v := range slices.Backward(comments) {
		group.prependComment(newUnitLine("", "# "+v, true))
	}
}

func (f *UnitFile) PrependUnitLine(groupName string, key string, value string) {
	var group *unitGroup
	if groupName == "" && len(f.groups) > 0 {
		group = f.groups[0]
	} else {
		// Uses magic "" for first comment-only group if no other groups
		group = f.ensureGroup(groupName)
	}
	group.prependLine(newUnitLine(key, value, false))
}

func (f *UnitFile) GetTemplateParts() (string, string, bool) {
	ext := filepath.Ext(f.Filename)
	basename := strings.TrimSuffix(f.Filename, ext)
	parts := strings.SplitN(basename, "@", 2)
	if len(parts) < 2 {
		return parts[0], "", false
	}
	return parts[0], parts[1], true
}

func (f *UnitFile) GetUnitDropinPaths() []string {
	unitName, instanceName, isTemplate := f.GetTemplateParts()

	ext := filepath.Ext(f.Filename)
	dropinExt := ext + ".d"

	dropinPaths := []string{}

	// Add top-level drop-in location (pod.d, container.d, etc)
	topLevelDropIn := strings.TrimPrefix(dropinExt, ".")
	dropinPaths = append(dropinPaths, topLevelDropIn)

	truncatedParts := strings.Split(unitName, "-")
	// If the unit contains any '-', then there are truncated paths to search.
	if len(truncatedParts) > 1 {
		// We don't need the last item because that would be the full path
		truncatedParts = truncatedParts[:len(truncatedParts)-1]
		// Truncated instance names are not included in the drop-in search path
		// i.e. template-unit@base-instance.service does not search template-unit@base-.service
		// So we only search truncations of the template name, i.e. template-@.service, and unit name, i.e. template-.service
		// or only the unit name if it is not a template.
		for i := range truncatedParts {
			truncatedUnitPath := strings.Join(truncatedParts[:i+1], "-") + "-"
			dropinPaths = append(dropinPaths, truncatedUnitPath+dropinExt)
			// If the unit is a template, add the truncated template name as well.
			if isTemplate {
				truncatedTemplatePath := truncatedUnitPath + "@"
				dropinPaths = append(dropinPaths, truncatedTemplatePath+dropinExt)
			}
		}
	}
	// For instanced templates, add the base template unit search path
	if instanceName != "" {
		dropinPaths = append(dropinPaths, unitName+"@"+dropinExt)
	}
	// Add the drop-in directory for the full filename
	dropinPaths = append(dropinPaths, f.Filename+".d")
	// Finally, reverse the list so that when drop-ins are parsed,
	// the most specific are applied instead of the most broad.
	// dropinPaths should be a list where the items are in order of specific -> broad
	// i.e., the most specific search path is dropinPaths[0], and broadest search path is dropinPaths[len(dropinPaths)-1]
	// Uses https://go.dev/wiki/SliceTricks#reversing
	for i := len(dropinPaths)/2 - 1; i >= 0; i-- {
		opp := len(dropinPaths) - 1 - i
		dropinPaths[i], dropinPaths[opp] = dropinPaths[opp], dropinPaths[i]
	}
	return dropinPaths
}

func PathEscape(path string) string {
	var escaped strings.Builder
	escapeString(&escaped, path, true)
	return escaped.String()
}
