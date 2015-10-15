package main

import (
	"errors"
	"flag"
	"fmt"
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
	srcdir      string
	executables []string
)

func clean(input string) (output string, err error) {
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

func main() {
	var err error

	log.SetFlags(0)
	flag.Parse()

	if *flagHelpShort || *flagHelp {
		flag.PrintDefaults()
		os.Exit(0)
	}

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

	gitignore := srcdir + string(os.PathSeparator) + ".gitignore"

	var gitignoreContentBytes []byte
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

		gitignoreContentBytes, err = ioutil.ReadFile(gitignore)
		if err != nil {
			log.Fatalln(gitignore, "unable to read", err)
		}
	}

	var gitIgnoreExecutables string
	if *flagClean {
		gitIgnoreExecutables, err = clean(string(gitignoreContentBytes))
		if err != nil {
			log.Fatalln("clean of gitignore failed:", err)
		}
	} else {
		filepath.Walk(srcdir, walkTree)

		sort.Strings(executables)
		gitIgnoreExecutables, err = insert(string(gitignoreContentBytes), strings.Join(executables, "\n"))
		if err != nil {
			log.Fatalln("insert to gitignore failed:", err)
		}
	}

	var outfile string
	var outfileMode os.FileMode

	if *flagStdout || *flagDryrun {
		fmt.Print(gitIgnoreExecutables)
	} else {
		outfile = gitignore
		if fGitignore != nil {
			gitignoreStat, statErr := fGitignore.Stat()
			if statErr != nil {
				log.Fatalln(gitignore, "unable to get stat", err)
			}
			outfileMode = gitignoreStat.Mode()
		} else {
			outfileMode = defaultMode
		}

		err = ioutil.WriteFile(outfile, []byte(gitIgnoreExecutables), outfileMode)
		if err != nil {
			log.Fatalln("write to", outfile, "failed:", err)
		}
	}
}

func walkTree(path string, info os.FileInfo, err error) error {
	// Skip .git directory tree, .gitignore and directories
	if strings.Contains(path, string(os.PathSeparator)+".git"+string(os.PathSeparator)) || strings.HasSuffix(path, ".gitignore") || info.IsDir() {
		return nil
	}

	var appendFile string

	// If -exec flag and file is executable
	appendFile = findExecutables(info, path)

	// If -gomain flag and file is go main
	appendFile = findGoMain(path)

	executablesAppend(appendFile)

	return nil
}

func findExecutables(info os.FileInfo, path string) (exe string) {
	var err error

	if *flagFindExecutables && info.Mode()&0111 > 0 {
		exe, err = filepath.Rel(srcdir, path)
		if err != nil {
			fmt.Println("filepath.Rel", err)
		}
	}
	return
}

func findGoMain(path string) (exe string) {
	if *flagFindGoMain && filepath.Ext(path) == ".go" {
		goContentBytes, err := ioutil.ReadFile(path)
		if err != nil {
			return
		}

		if strings.Contains(string(goContentBytes), "package main\n") {
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
		// Add file only once
		for _, exe := range executables {
			if exe == appendFile {
				return
			}
		}
		executables = append(executables, appendFile)
	}

}
