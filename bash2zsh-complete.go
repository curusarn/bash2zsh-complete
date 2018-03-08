// Reading and writing files are basic tasks needed for
// many Go programs. First we'll look at some examples of
// reading files.

package main

import (
    "strings"
    "io/ioutil"
    "os"
    "log"
    "regexp"
    "fmt"
)

// Reading files requires checking most calls for errors.
// This helper will streamline our error checks below.
func check(e error) {
    if e != nil {
        panic(e)
    }
}

func GetIndent(original_line string) string {
    return strings.Repeat(" ", len(original_line) - len(strings.TrimLeft(original_line, " ")))
}

func main() {

    log := log.New(os.Stderr, "", log.LstdFlags)
    log.Println("Log init")

    if len(os.Args) < 2 {
        log.Printf("Found <%v> arguments (1 required)\n", len(os.Args) - 1)
        log.Printf("USAGE: go run bash2zsh-complete BASH_COMPLETION_FILE [> ZSH_COMPLETION_FILE]\n")
    }
    dat, err := ioutil.ReadFile(os.Args[1])
    check(err)

    input := strings.Split(string(dat),"\n")

    // FIND and REPLACE: COMPREPLY=`compgen ... "$arg" ...` <with> compadd `echo $arg`

    // FIND: complete -F _fname cmdname
    func_name := ""
    cmd_name := ""
    var complete_index int

    log.Println("Looking for 'complete' command ...")
    for index, line := range input {
        trim_line := strings.TrimSpace(line)
        if !strings.HasPrefix(trim_line, "complete") {
            continue
        }
        log.Printf("Found <%v> command on line <%v>\n", trim_line, index)

        if func_name != "" {
            log.Printf("ERR: More than one 'complete' command (1 required) - exiting!\n")
            return
        }

        words := strings.Fields(trim_line)
        if len(words) != 4 {
            log.Printf("ERR: 'complete' command has <%v> words (4 required) - exiting!\n", len(words))
            return
        }

        if words[1] == "-F" {
            func_name = words[2]
            cmd_name = words[3]
            complete_index = index
            continue
        } else if words[2] == "-F" {
            func_name = words[3]
            cmd_name = words[1]
            complete_index = index
            continue
        } else {
            log.Printf("ERR: No '-F' option in 'complete' complete command (only 'complete -F' is supported) - exiting!\n", len(words))
            return
        }
    }

    if func_name == "" || cmd_name == "" {
        log.Printf("ERR: 'complete' command not found - exiting!\n")
        return
    }
    log.Printf("Discovered completion function <%v> for command <%v>\n", func_name, cmd_name)

    var output []string
    // ADD first line: #compdef cmdname _fname
    var compdef_line string
    if func_name[1:] == cmd_name {
        compdef_line = "#compdef " + cmd_name
    } else {
        compdef_line = "#compdef " + func_name + " " + cmd_name
    }
    log.Printf("Adding #compdef zsh directive <%v>\n", compdef_line)
    output = append(output, compdef_line)
    output = append(output, "# bash2zsh This file was generated using bash2zsh-complete (https://github.com/curusarn/bash2zsh-complete)")

    // FIND: _fname() {
    log.Printf("Looking for <%v> function definition ...\n", func_name)
    i := 0
    r := regexp.MustCompile(func_name + `[[:space:]]*\([[:space:]]*\)[[:space:]]*{`)
    trim_line := ""
    for {
        if i >= len(input) {
            log.Printf("ERR: Cannot find <%v> function (line starting with '%v' required, 'function %v' syntax is not supported) - exiting!\n", func_name, func_name, func_name)
            return
        }
        output = append(output, input[i])
        trim_prev_line := trim_line
        trim_line = strings.TrimSpace(input[i])
        //log.Printf("1: <%v>\n", trim_line)
        if r.MatchString(trim_line) {
            log.Printf("Found <%v> function on line <%v>\n", trim_line, i)
            break
        }
        trim_two_input := trim_prev_line + trim_line
        //log.Printf("2: <%v>\n", trim_two_input)
        if r.MatchString(trim_two_input) {
            log.Printf("Found <%v> function on line <%v>\n", trim_prev_line, i-1)
            break
        }
        i++
    }
    i++

    // POPULATE bash variables using zsh variables
    output = append(output, "    # bash2zsh initialize bash variables \n    local COMP_WORDS COMP_CWORD\n    COMP_WORDS=($words[2,-1])\n    COMP_CWORD=$(($CURRENT-1))\n")
    log.Printf("Adding bash variable initialization at line <%v>\n", i)

    log.Printf("Processing file...\n")
    for i < len(input) {
        line := input[i]
        trim_line := strings.TrimSpace(input[i])
        if i == complete_index {
            log.Printf("Commenting-out <%v> command at line <%v>\n", trim_line, i)
            line = "# bash2zsh comment-out complete command\n#" + line

        } else if strings.HasPrefix(trim_line, "COMPREPLY=") {
            log.Printf("Found line <%v> starting with 'COMPREPLY=' at <%v>\n", trim_line, i)
            if trim_line == "COMPREPLY=()" { 
                // NOTE: I don't think bash allows spaces anywhere in this^^^ statement
                log.Printf("Skipping empty assignment <%v> at <%v>\n", trim_line, i)
            } else  {
                log.Printf("Looking for word-list...\n")
                wordlist_char_index := strings.Index(trim_line, "-W")
                if wordlist_char_index == -1 {
                    log.Printf("ERR: Cannot find 'compgen -W' option at <%v> (using compgen with -W option is required)\n", i)
                    return
                }
                log.Printf("Found 'compgen -W' option at <%v>\n", i)
                wordlist_char_index += 2 // skip '-W' (two chars)
                wordlist_tmp := strings.TrimSpace(trim_line[wordlist_char_index:])
                wordlist_quote := string(wordlist_tmp[0])
                allowed_quotes := "\"'"
                var wordlist string
                if strings.Contains(allowed_quotes, wordlist_quote) {
                    log.Printf("Found word-list quotes <%v> at <%v>\n", wordlist_quote, i)
                    end_index := strings.Index(wordlist_tmp[1:], wordlist_quote) + 1
                    wordlist = wordlist_tmp[1:end_index]
                } else {
                    log.Printf("WARN: Cannot find wordlist quotes at <%v>\n", i)
                    wordlist = strings.Fields(wordlist_tmp)[0]
                }
                compadd_line := "compadd \"$@\" `echo " + wordlist + "`"
                log.Printf("Replacing 'COMPREPLY' with 'compadd' - <%v> with <%v> at <%v>\n", trim_line, compadd_line, i)
                indent := GetIndent(line)
                line = indent + "# bash2zsh replace COMPREPLY with compadd\n" + indent + "#" + trim_line + "\n" + indent + compadd_line 
            }
        }
        output = append(output, line)
        i++
    }

    for _, line := range output {
        fmt.Println(line)
    }
}

