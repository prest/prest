package postgres

import (
	"fmt"
	"log"
	"os"
)

func createMockScripts(base string) {
	log.Println("Create scripts on", base)
	err := os.MkdirAll(fmt.Sprint(base, "/fulltable"), 0777)
	if err != nil {
		log.Println(err)
	}
	_, err = os.Create(fmt.Sprint(base, "/fulltable/get_all.read.sql"))
	if err != nil {
		log.Println(err)
	}
	_, err = os.Create(fmt.Sprint(base, "/fulltable/get_all_slice.read.sql"))
	if err != nil {
		log.Println(err)
	}
	_, err = os.Create(fmt.Sprint(base, "/fulltable/write_all.write.sql"))
	if err != nil {
		log.Println(err)
	}
	_, err = os.Create(fmt.Sprint(base, "/fulltable/create_table.write.sql"))
	if err != nil {
		log.Println(err)
	}
	_, err = os.Create(fmt.Sprint(base, "/fulltable/patch_all.update.sql"))
	if err != nil {
		log.Println(err)
	}
	_, err = os.Create(fmt.Sprint(base, "/fulltable/put_all.update.sql"))
	if err != nil {
		log.Println(err)
	}
	_, err = os.Create(fmt.Sprint(base, "/fulltable/delete_all.delete.sql"))
	if err != nil {
		log.Println(err)
	}
}

func writeMockScripts(base string) {
	base = fmt.Sprint(base, "/fulltable/")
	log.Println("Write scripts on", base)

	write := func(sql, fileName string) {
		var file, err = os.OpenFile(fmt.Sprint(base, fileName), os.O_RDWR, 0644)
		if err != nil {
			log.Fatal(err)
		}
		defer file.Close()

		// write some text to file
		_, err = file.WriteString(sql)
		if err != nil {
			log.Fatal(err)
		}
		// save changes
		err = file.Sync()
		if err != nil {
			log.Fatal(err)
		}
	}

	write("SELECT * FROM test7 WHERE name = '{{.field1}}'", "get_all.read.sql")
	write(`SELECT * FROM test7 WHERE name IN {{inFormat "field1"}}`, "get_all_slice.read.sql")
	write("INSERT INTO test7 (name, surname) VALUES ('{{.field1}}', '{{.field2}}')", "write_all.write.sql")
	write("CREATE TABLE {{.field1}};", "create_table.write.sql")
	write("UPDATE test7 SET name = '{{.field1}}' WHERE surname = '{{.field2}}'", "patch_all.update.sql")
	write("UPDATE test7 SET surname = '{{.field1}}' WHERE name = '{{.field2}}'", "put_all.update.sql")
	write("DELETE FROM test7 WHERE name = '{{.field1}}'", "delete_all.delete.sql")
}

func removeMockScripts(base string) {
	log.Println("Remove scripts on", base)
	err := os.RemoveAll(base)
	if err != nil {
		log.Println(err)
	}
}
