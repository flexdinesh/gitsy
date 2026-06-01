package status

import (
	"regexp"
	"strconv"
	"strings"
)

type BranchStatus struct {
	Raw      string
	Name     string
	Upstream string
	Ahead    int
	Behind   int
	Gone     bool
	Metadata string
}

type Category string

const (
	Modified  Category = "modified"
	Staged    Category = "staged"
	Untracked Category = "untracked"
	Deleted   Category = "deleted"
	Renamed   Category = "renamed"
	Conflict  Category = "conflict"
	Other     Category = "other"
)

type Item struct {
	Raw      string
	Code     string
	Path     string
	Category Category
}

type Parsed struct {
	Raw     string
	Branch  *BranchStatus
	Items   []Item
	Changed bool
}

var (
	metadataPattern = regexp.MustCompile(`\s\[(.+)]$`)
	countPatterns   = map[string]*regexp.Regexp{
		"ahead":  regexp.MustCompile(`ahead (\d+)`),
		"behind": regexp.MustCompile(`behind (\d+)`),
	}
	conflictCodes = map[string]struct{}{
		"DD": {},
		"AU": {},
		"UD": {},
		"UA": {},
		"DU": {},
		"AA": {},
		"UU": {},
	}
)

func HasChangesOrDivergence(raw string) bool {
	for _, line := range splitLines(raw) {
		if strings.HasPrefix(line, "## ") {
			if strings.Contains(line, "[") && strings.Contains(line, "]") {
				return true
			}
			continue
		}
		if line != "" {
			return true
		}
	}
	return false
}

func CanFastForward(parsed Parsed) bool {
	if parsed.Branch == nil || parsed.Branch.Name == "" {
		return false
	}
	return parsed.Branch.Upstream != "" &&
		!parsed.Branch.Gone &&
		parsed.Branch.Behind > 0 &&
		parsed.Branch.Ahead == 0 &&
		len(parsed.Items) == 0
}

func Parse(raw string) Parsed {
	lines := []string{}
	for _, line := range splitLines(raw) {
		if line != "" {
			lines = append(lines, line)
		}
	}

	var branch *BranchStatus
	items := []Item{}
	for _, line := range lines {
		if strings.HasPrefix(line, "## ") {
			parsedBranch := ParseBranchLine(line)
			branch = &parsedBranch
			continue
		}
		items = append(items, ParseStatusLine(line))
	}

	return Parsed{
		Raw:     raw,
		Branch:  branch,
		Items:   items,
		Changed: HasChangesOrDivergence(raw),
	}
}

func ParseBranchLine(line string) BranchStatus {
	raw := line
	content := line
	if strings.HasPrefix(content, "## ") {
		content = strings.TrimPrefix(content, "## ")
	}

	metadata := ""
	branchPart := content
	if match := metadataPattern.FindStringSubmatchIndex(content); match != nil {
		metadata = content[match[2]:match[3]]
		branchPart = strings.TrimRight(content[:match[0]], " ")
	}

	name := branchPart
	upstream := ""
	if index := strings.Index(branchPart, "..."); index != -1 {
		name = branchPart[:index]
		upstream = branchPart[index+3:]
	}

	return BranchStatus{
		Raw:      raw,
		Name:     name,
		Upstream: upstream,
		Ahead:    parseMetadataCount(metadata, "ahead"),
		Behind:   parseMetadataCount(metadata, "behind"),
		Gone:     strings.Contains(metadata, "gone"),
		Metadata: metadata,
	}
}

func ParseStatusLine(line string) Item {
	code := line
	if len(code) > 2 {
		code = line[:2]
	}
	filePath := ""
	if len(line) > 3 {
		filePath = line[3:]
	}

	return Item{
		Raw:      line,
		Code:     code,
		Path:     filePath,
		Category: categorize(code, filePath),
	}
}

func categorize(code string, filePath string) Category {
	if _, ok := conflictCodes[code]; ok || strings.Contains(code, "U") {
		return Conflict
	}
	if code == "??" {
		return Untracked
	}
	if strings.Contains(code, "R") || strings.Contains(filePath, " -> ") {
		return Renamed
	}
	if strings.Contains(code, "D") {
		return Deleted
	}

	indexStatus := byte(' ')
	worktreeStatus := byte(' ')
	if len(code) > 0 {
		indexStatus = code[0]
	}
	if len(code) > 1 {
		worktreeStatus = code[1]
	}

	if indexStatus != ' ' && indexStatus != '?' {
		return Staged
	}
	if worktreeStatus != ' ' {
		return Modified
	}
	return Other
}

func parseMetadataCount(metadata string, key string) int {
	if metadata == "" {
		return 0
	}
	match := countPatterns[key].FindStringSubmatch(metadata)
	if len(match) < 2 {
		return 0
	}
	parsed, err := strconv.Atoi(match[1])
	if err != nil {
		return 0
	}
	return parsed
}

func splitLines(raw string) []string {
	return strings.Split(strings.ReplaceAll(raw, "\r\n", "\n"), "\n")
}
