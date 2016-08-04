package main

import (
	"log"
	"os"
	//"io"
	"encoding/binary"
	"fmt"

	"github.com/driusan/gpt"
)

func main() {
	// TODO: Read this from the command line
	if len(os.Args) < 3 {
		fmt.Fprintf(os.Stderr,
			`Usage: %s disk action

disk is the file of the block device on your operating system (ie. /dev/sda)
and action is the subcommand to run.

Valid actions are:
	verify	verifies that the installed GPT table is valid
	show  	shows the GPT table currently installed

Note that only 512 logical block sizes are currently supported.
`, os.Args[0])
		os.Exit(2)
	}

	// Open the block device
	f, err := os.Open(os.Args[1])
	if err != nil {
		log.Fatalln(err.Error())
	}
	defer f.Close()

	// Attempt to read one block. This is the protective MBR and gets
	// thrown away. TODO: Actually read the MBR
	header := gpt.GPTHeader{}
	err = binary.Read(f, binary.LittleEndian, &header)
	if err != nil {
		log.Fatalf("%v", err)
	}

	// Read the real GPT header
	err = binary.Read(f, binary.LittleEndian, &header)
	if err != nil {
		log.Fatalf("%v", err)
	}

	switch cmd := os.Args[2]; cmd {
	case "verify":
		err = header.Verify()
		if err != nil {
			log.Fatalln(err.Error())
		}
		fmt.Printf("GPT appears to be valid.\n")
		os.Exit(0)
	case "show":
		// Still need to verify the header
		err = header.Verify()
		if err != nil {
			log.Fatalln(err.Error())
		}

		// Print a header line with the same formatting width as the
		// print statements

		partitions, err := header.GetPartitions(f)
		if err != nil {
			log.Fatalln(err.Error())
		}

		fmt.Printf("%11s %11s %5s %s\n", "Start", "Size", "Index", "Contents")
		for i, p := range partitions {
			if p.PartitionType != gpt.ZeroGUID {
				if name := p.GetName(); name != "" {
					fmt.Printf("%11d %11d %5d %s (Part name: %s)\n", p.StartingLBA, p.Size(), i, p.PartitionType.HumanString(), name)
				} else {
					fmt.Printf("%11d %11d %5d %s\n", p.StartingLBA, p.Size(), i, p.PartitionType.HumanString())
				}
			}
		}
	}

}
