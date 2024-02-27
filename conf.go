package main

import (
	"bufio"
	"log"
	"os"
	"strings"
)

type (
	command string

	server struct {
		Name    string
		Host    string
		User    string
		Keypath string
		Output  string // sshOutputArray
	}

	servers  []server
	commands []command

	config struct {
		Servers  servers
		Commands commands
	}
)

func (c *commands) getCommandFromConfig(commandConfig string) error {
	file, err := os.Open(commandConfig)
	if err != nil {
		log.Fatal(err)
		return err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		*c = append(*c, command(line))
	}

	if err := scanner.Err(); err != nil {
		log.Fatal(err)
		return err
	}

	// Теперь commands содержит команды из файла commands.csv
	return nil

}

func (s *servers) getServersFromConfig(serversConfig string) error {
	file, err := os.Open(serversConfig)
	if err != nil {
		log.Fatal(err)
		return err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		fields := strings.Split(line, ", ")
		if len(fields) != 3 {
			log.Fatalf("Invalid line: %s", line)
			return err
		}

		*s = append(*s, server{
			Host:    fields[0],
			User:    fields[1],
			Keypath: fields[2],
		})
	}

	if err := scanner.Err(); err != nil {
		log.Fatal(err)
		return err
	}

	// Теперь servers содержит информацию из файла servers.conf
	return nil
}

//func (c *config) runOnServer() error {
//	//wg.Add(1)
//	//wg.Wait()
//	//wg.Done()
//}
