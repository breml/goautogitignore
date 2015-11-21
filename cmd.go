package main

import (
	"errors"
	"flag"
	"fmt"
	"go/parser"
	"go/token"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const (
	delimiterName            string      = "gogitignore"
	delimiterStartIdentifier string      = "start"
	delimiterEndIdentifier   string      = "end"
	comment                  string      = "#"
	delimiterStart                       = "\n" + comment + " " + delimiterName + " " + delimiterStartIdentifier + "\n"
	delimiterEnd                         = comment + " " + delimiterName + " " + delimiterEndIdentifier + "\n"
	defaultMode              os.FileMode = 0644
)

var (
	flagHelpShort       = flag.Bool("h", false, "print usage")
	flagHelp            = flag.Bool("help", false, "print usage")
	flagSrcDir          = flag.String("dir", ".", "destination directory where .gitignore is located and where to traverse directory tree for go programs.")
	flagFindExecutables = flag.Bool("exec", false, "find all files with executable bit set")
	flagFindGoMain      = flag.Bool("gomain", true, "add executables, resulting from building go main packages")
	flagStdout          = flag.Bool("stdout", false, "print resulting .gitignore to stdout instead of updating .gitignore in place")
	flagDryrun          = flag.Bool("dryrun", false, "dryrun, no changes are made")
	flagClean           = flag.Bool("clean", false, "clean everything between gogitignore start and end markers")
)

var (
	srcdir          string
	executables     []string
	executablesHash map[string]bool
	fset            *token.FileSet
)

func main() {
	var err error

	log.SetFlags(0)
	flag.Parse()

	if *flagHelpShort || *flagHelp {
		flag.PrintDefaults()
		os.Exit(0)
	}

	gitignore := locateGitignore()

	gitignoreContent, outfileMode := readGitignore(gitignore)

	var gitIgnoreExecutables string
	if *flagClean {
		gitIgnoreExecutables, err = cleanGitignore(gitignoreContent)
		if err != nil {
			log.Fatalln("clean of gitignore failed:", err)
		}
	} else {
		gitIgnoreExecutables = updateGitignore(gitignoreContent)
	}

	if *flagStdout || *flagDryrun {
		fmt.Print(gitIgnoreExecutables)
	} else {
		err = ioutil.WriteFile(gitignore, []byte(gitIgnoreExecutables), outfileMode)
		if err != nil {
			log.Fatalln("write to", gitignore, "failed:", err)
		}
	}
}

func locateGitignore() (gitignore string) {
	var err error

	srcdir, err = filepath.Abs(filepath.Clean(*flagSrcDir))
	if err != nil {
		log.Fatalln(err)
	}

	fDstdir, err := os.Open(srcdir)
	if err != nil {
		if os.IsNotExist(err) {
			log.Fatalln(err)
		} else {
			log.Fatalln(err)
		}
	}
	defer func() {
		err = fDstdir.Close()
		if err != nil {
			log.Fatalln(err)
		}
	}()

	_, err = fDstdir.Readdir(1)
	if err != nil {
		log.Fatalln(srcdir, "is not a directory")
	}

	return srcdir + string(os.PathSeparator) + ".gitignore"
}

func readGitignore(gitignore string) (gitignoreContent string, gitignoreFilemode os.FileMode) {
	var err error

	gitignoreFilemode = defaultMode
	fGitignore, err := os.Open(gitignore)
	if err != nil {
		if os.IsNotExist(err) {
			log.Println(gitignore, "does not exists, create new file")
		} else {
			log.Fatalln(gitignore, "not readable", err)
		}
	} else {
		defer func() {
			err = fGitignore.Close()
			if err != nil {
				log.Fatalln(err)
			}
		}()

		gitignoreContentBytes, err := ioutil.ReadFile(gitignore)
		if err != nil {
			log.Fatalln(gitignore, "unable to read", err)
		}
		gitignoreContent = string(gitignoreContentBytes)

		gitignoreStat, statErr := fGitignore.Stat()
		if statErr != nil {
			log.Fatalln(gitignore, "unable to get stat", err)
		}
		gitignoreFilemode = gitignoreStat.Mode()

	}

	return gitignoreContent, gitignoreFilemode
}

func cleanGitignore(input string) (output string, err error) {
	if len(input) == 0 {
		return input, nil
	}

	if strings.Contains(input, delimiterStart) {
		if strings.Count(input, delimiterStart) > 1 {
			return input, errors.New("multiple instances of start delimiter")
		}
		if strings.Contains(input, delimiterEnd) {
			if strings.Count(input, delimiterEnd) > 1 {
				return input, errors.New("multiple instances of closing delimiter")
			}
			startPos := strings.Index(input, delimiterStart)
			endPos := strings.Index(input, delimiterEnd) + len(delimiterEnd)

			if startPos-2 >= 0 && input[startPos-2] == '\n' {
				startPos--
			}
			if endPos+1 < len(input) && input[endPos+1] == '\n' {
				endPos++
			}

			output = input[:startPos] + input[endPos:]
		} else {
			return input, errors.New("found no closing delimiter")
		}
	} else {
		output = input
	}

	return output, nil
}

func updateGitignore(gitignoreContent string) string {
	var err error

	executablesHash = make(map[string]bool)

	fset = token.NewFileSet()
	err = filepath.Walk(srcdir, walkTree)
	if err != nil {
		log.Fatalln("directory walk failed:", err)
	}

	for executable := range executablesHash {
		fmt.Println(executable)
		executables = append(executables, executable)
	}

	sort.Strings(executables)
	gitIgnoreExecutables, err := insert(gitignoreContent, strings.Join(executables, "\n"))
	if err != nil {
		log.Fatalln("insert to gitignore failed:", err)
	}
	return gitIgnoreExecutables
}

func insert(input string, addition string) (output string, err error) {
	if len(addition) == 0 {
		return input, nil
	}

	if !strings.HasSuffix(addition, "\n") {
		addition = addition + "\n"
	}
	addition = delimiterStart + addition + delimiterEnd
	if len(input) == 0 {
		return addition, nil
	}

	if strings.Contains(input, delimiterStart) {
		if strings.Count(input, delimiterStart) > 1 {
			return input, errors.New("multiple instances of start delimiter")
		}

		if strings.Contains(input, delimiterEnd) {
			if strings.Count(input, delimiterEnd) > 1 {
				return input, errors.New("multiple instances of closing delimiter")
			}
			if !strings.HasSuffix(input, "\n") {
				input = input + "\n"
			}

			startPos := strings.Index(input, delimiterStart)
			endPos := strings.Index(input, delimiterEnd) + len(delimiterEnd)

			output = input[:startPos] + addition + input[endPos:]

		} else {
			return input, errors.New("found no closing delimiter")
		}
	} else {
		if !strings.HasSuffix(input, "\n") {
			input = input + "\n"
		}
		output = input + addition
	}

	return output, nil
}

func walkTree(path string, info os.FileInfo, err error) error {
	// Skip .git directory tree, .gitignore and directories
	if strings.Contains(path, string(os.PathSeparator)+".git"+string(os.PathSeparator)) || strings.HasSuffix(path, ".gitignore") || info.IsDir() {
		return nil
	}

	var appendFile string

	// If -exec flag and file is executable
	if *flagFindExecutables {
		appendFile = findExecutables(info, path)
	}

	// If -gomain flag and file is go main
	if *flagFindGoMain {
		appendFile = findGoMain(path)
	}

	executablesAppend(appendFile)

	return nil
}

func findExecutables(info os.FileInfo, path string) (exe string) {
	var err error

	if info.Mode()&0111 > 0 {
		exe, err = filepath.Rel(srcdir, path)
		if err != nil {
			fmt.Println("filepath.Rel", err)
		}
	}
	return
}

func findGoMain(path string) (exe string) {
	if filepath.Ext(path) == ".go" {
		f, err := parser.ParseFile(fset, path, nil, parser.PackageClauseOnly)
		if err != nil {
			fmt.Println(path, "parse error", err)
		}
		if f.Name.Name == "main" {
			dir := filepath.Dir(path)
			exec := dir[strings.LastIndex(dir, string(filepath.Separator))+1:]
			exe, err = filepath.Rel(srcdir, dir+string(filepath.Separator)+exec)
			if err != nil {
				fmt.Println("filepath.Rel", err)
			}
		}
	}
	return
}

func executablesAppend(appendFile string) {
	if len(appendFile) > 0 {
		executablesHash[appendFile] = true
	}
}
