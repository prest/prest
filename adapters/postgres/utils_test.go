package postgres

import (
	"fmt"
	"log"
	"os"
)

func createMockScripts(base string) {
	log.Println("Create scripts on", base)
	err := os.Mkdir(fmt.Sprint(base, "/fulltable"), 0777)
	if err != nil {
		log.Println(err)
	}
	_, err = os.Create(fmt.Sprint(base, "/fulltable/get_all.read.sql"))
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

	write("SELECT * FROM test6 WHERE name = {{.Field1}}", "get_all.read.sql")
	write("INSERT INTO test6 (name, surname) VALUES ({{.Field1}}, {{.Field2}}) RETURNING id", "write_all.write.sql")
	write("CREATE TABLE {{.Field1}};", "create_table.write.sql")
	write("UPDATE test6 SET name = {{.Field1}} WHERE surname = {{.Field2}}", "patch_all.update.sql")
	write("UPDATE test6 SET surname = {{.Field1}} WHERE name = {{.Field2}}", "put_all.update.sql")
	write("DELETE FROM test6 WHERE name = {{.Field1}}", "delete_all.delete.sql")
}

func removeMockScripts(base string) {
	log.Println("Remove scripts on", base)
	err := os.RemoveAll(base)
	if err != nil {
		log.Println(err)
	}
}
