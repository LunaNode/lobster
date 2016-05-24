package main

import "github.com/LunaNode/lobster"

import "github.com/mattes/migrate/migrate"
import "github.com/mattes/migrate/migrate/direction"
import "github.com/mattes/migrate/file"
import pipep "github.com/mattes/migrate/pipe"
import _ "github.com/mattes/migrate/driver/mysql"

import "fmt"
import "os"

func main() {
	cfgPath := "lobster.cfg"
	if len(os.Args) >= 2 {
		cfgPath = os.Args[1]
	}
	lobster.Setup(cfgPath)
	url := "mysql://" + lobster.GetDatabaseString()
	pipe := pipep.New()
	go migrate.Up(pipe, url, "./db/migrations")
	hadError := false
	done := false
	for !done {
		select {
		case item, more := <-pipe:
			if !more {
				done = true
			} else {
				switch item.(type) {
				case string:
					fmt.Println(item.(string))
				case error:
					fmt.Printf("ERROR: %v\n", item)
					hadError = true
				case file.File:
					f := item.(file.File)
					if f.Direction == direction.Up {
						fmt.Print("> ")
					} else if f.Direction == direction.Down {
						fmt.Print("< ")
					}
					fmt.Println(f.FileName)
				default:
					fmt.Printf("%v", item)
				}
			}
		}
	}
	if !hadError {
		fmt.Println("database up to date")
	}
}
