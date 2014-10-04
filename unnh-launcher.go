package main

import (
    "fmt"
    "log"
    "os"
    "path/filepath"
    "os/exec"
    "time"
    "io"
    "syscall"
)


const ROOT = "/"
const DEFAULT_GAMEDIR = "unnethack.49"
const LOGFILE = "/go-debug.log"
const BINARY = "./unnethack"
const UID = 3000

var logger *log.Logger

func main() {
    logfd := createLogger()
	defer logfd.Close()
    logger.Print("starting")
    
    setCoreSize()
    logger.Printf("set core size")
    
    if len(os.Args) != 3 {
        logger.Fatal("invalid number of arguments: ", len(os.Args))
    }
	region := os.Args[1]
	if (region != "eu" && region != "us") {
		logger.Fatal("invalid region: ", region)
	}
	logger.Print("region: ", region)
    username := os.Args[2]
    logger.Print("username: ", username)

    gamedir := getGamedir(username)
    logger.Print("gamedir: ", gamedir)

    cmd := startGame(gamedir, username)
    logger.Print("waiting for game to finish...")
    err := cmd.Wait()
    if err == nil {
        logger.Print("game finished normally")
    } else {
        logger.Printf("game finished with error: %v", err)

		fd, err2 := os.OpenFile("/var/unnethack/livelog", os.O_APPEND | os.O_WRONLY | os.O_SYNC, 0660)
		if err2 != nil {
			logger.Fatal(err2)
		}
		defer fd.Close()
		fd.WriteString(fmt.Sprintf("player=%s:crash=%v\n", username, err))
    }

	moveDumpfiles(username, region)
}

func setCoreSize() {
    var lim syscall.Rlimit
    err := syscall.Getrlimit(syscall.RLIMIT_CORE, &lim)
    if err != nil {
        logger.Fatal(err)
    }
    // wtf is this?
    lim.Max = ^uint64(0)
    lim.Cur = ^uint64(0)
    err = syscall.Setrlimit(syscall.RLIMIT_CORE, &lim)
    if err != nil {
        logger.Fatal(err)
    }
    err = syscall.Getrlimit(syscall.RLIMIT_CORE, &lim)
    if err != nil {
        logger.Fatal(err)
    }
    logger.Printf("set core size")
}

func createLogger() *os.File {
    logfd, err := os.OpenFile(LOGFILE, os.O_APPEND | os.O_WRONLY | os.O_SYNC | os.O_CREATE, 0660)
    if err != nil {
        panic(err)
    }
    
    logger = log.New(logfd, fmt.Sprintf("%d ", os.Getpid()), log.Ldate | log.Ltime | log.Lshortfile)
	return logfd
}

func getGamedir(username string) string {
    chdir(ROOT)
    files, err := filepath.Glob("./unnethack*")
    if err != nil {
        logger.Fatal(err)
    }
    for _, fn := range files {
        var extensions = []string{"", ".gz", "bz2"}
        for _, ext := range extensions {
            var path = fmt.Sprintf("%s%s/var/save/%d%s%s", ROOT, fn, UID, username, ext)
            if exists(path) {
                doBackup(path)
                return fn
            }
        }
    }
    return DEFAULT_GAMEDIR
}

func doBackup(path string)  {
    dest := fmt.Sprintf("%s.%s.bak", path, time.Now().Format(time.RFC3339))
    logger.Printf("backing up '%s' -> '%s'", path, dest)
    err := cp(path, dest)
    if err != nil {
        logger.Fatal(err)
    }
}

func startGame(gamedir, username string) *exec.Cmd {    
    chdir(fmt.Sprintf("%s/%s", ROOT, gamedir))
    logger.Print("starting game...")
    cmd := exec.Command(BINARY, "-u", username)
    cmd.Stdin = os.Stdin
    cmd.Stdout = os.Stdout
    cmd.Stderr = os.Stderr
    err := cmd.Start()
    if err != nil {
        logger.Fatal(err)
    }
    return cmd
}

func moveDumpfiles(username, region string) {
	var extensions = []string{".txt", ".txt.html"}
	for _, ext := range extensions {
		var path = fmt.Sprintf("/var/unnethack/dumps/%s%s", username, ext)
		logger.Print("checking: ", path)
		info, err := os.Stat(path)
		if err == nil {
			time := info.ModTime().Unix()
			dest := fmt.Sprintf("/users/%s/dumps/%s/%s.%d%s", username, region, username, time, ext)
			logger.Printf("moving dump '%s' -> '%s'", path, dest)
			err = mv(path, dest)
			if err != nil {
				logger.Fatal(err)
			}
			last := fmt.Sprintf("/users/%s/dumps/%s/%s.last%s", username, region, username, ext)
			os.Remove(last)
			chdir(fmt.Sprintf("/users/%s/dumps/%s/", username, region))
			err = os.Symlink(fmt.Sprintf("%s.%d%s", username, time, ext),
				fmt.Sprintf("%s.last%s", username, ext))
			if err != nil {
				logger.Fatal(err)
			}
		}
	}
}

func exists(fn string) bool {
    _, err := os.Stat(fn)
    return err == nil
}

func mv(src, dst string) error {
    s, err := os.Open(src)
    if err != nil {
        return err
    }
    defer s.Close()
    d, err := os.Create(dst)
    if err != nil {
        return err
    }
    if _, err := io.Copy(d, s); err != nil {
        d.Close()
        return err
    }
	err = os.Remove(src)
	if err != nil {
		return err
	}
    return d.Close()
}

func cp(src, dst string) error {
    s, err := os.Open(src)
    if err != nil {
        return err
    }
    defer s.Close()
    d, err := os.Create(dst)
    if err != nil {
        return err
    }
    if _, err := io.Copy(d, s); err != nil {
        d.Close()
        return err
    }
    return d.Close()
}

func chdir(dir string) {
    err := os.Chdir(dir)
    if err != nil {
        logger.Fatal(err)
    }
}
