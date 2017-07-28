// Copyright (c) 2012-2013 Jason McVetta.  This is Free Software, released under
// the terms of the GPL v3.  See http://www.gnu.org/copyleft/gpl.html for details.
// Resist intellectual serfdom - the ownership of ideas is akin to slavery.

package neoism

import "testing"

func benchCleanup(b *testing.B, db *Database) {
	qs := []*CypherQuery{
		&CypherQuery{
			Statement: `START r=rel(*) DELETE r`,
		},
		&CypherQuery{
			Statement: `START n=node(*) DELETE n`,
		},
	}
	err := db.CypherBatch(qs)
	if err != nil {
		b.Fatal(err)
	}
}

func BenchmarkNodeChain(b *testing.B) {
	b.StopTimer()
	db := connectBench(b)
	b.StartTimer()
	n0, _ := db.CreateNode(Props{"name": 0})
	defer n0.Delete()
	lastNode := n0
	for i := 1; i < b.N; i++ {
		nextNode, _ := db.CreateNode(Props{"name": i})
		defer nextNode.Delete()
		r0, _ := lastNode.Relate("knows", nextNode.Id(), Props{"name": i})
		defer r0.Delete()
	}
	b.StopTimer()
}

func BenchmarkNodeChainBatch(b *testing.B) {
	b.StopTimer()
	db := connectBench(b)
	b.StartTimer()
	qs := []*CypherQuery{}
	nodes := []int{}
	rels := []int{}
	cq := CypherQuery{
		Statement:  `CREATE (n:Person {name: {i}}) RETURN n`,
		Parameters: Props{"i": 0},
		Result:     &[]Node{},
	}
	qs = append(qs, &cq)
	nodes = append(nodes, 0)
	for i := 1; i < b.N; i++ {
		cq0 := CypherQuery{
			Statement:  `CREATE (n:Person {name: {i}}) RETURN n`,
			Parameters: Props{"i": i},
			Result:     &[]Node{},
		}
		qs = append(qs, &cq0)
		nodes = append(nodes, i)
		cq1 := CypherQuery{
			Statement:  `MATCH a:Person, b:Person WHERE a.name = {i} AND b.name = {k} CREATE a-[r:Knows {name: {i}}]->b RETURN id(r)`,
			Parameters: Props{"i": i, "k": i - 1},
			Result:     &[]Relationship{},
		}
		qs = append(qs, &cq1)
		rels = append(rels, i)
	}
	err := db.CypherBatch(qs)
	if err != nil {
		b.Fatal(err)
	}
	b.StopTimer()
	//
	// Cleanup
	//
	qs = []*CypherQuery{}
	for _, r := range rels {
		cq := CypherQuery{
			Statement:  `MATCH ()-[r:Knows]->() WHERE r.name = {i} DELETE r`,
			Parameters: Props{"i": r},
		}
		qs = append(qs, &cq)
	}
	for _, n := range nodes {
		cq := CypherQuery{
			Statement:  `MATCH n:Person WHERE n.name = {i} DELETE n`,
			Parameters: Props{"i": n},
		}
		qs = append(qs, &cq)
	}
	err = db.CypherBatch(qs)
	if err != nil {
		b.Fatal(err)
	}
}

func BenchmarkNodeChainTx10____(b *testing.B) {
	nodeChainTx(b, 10)
}

func BenchmarkNodeChainTx100___(b *testing.B) {
	nodeChainTx(b, 100)
}

func BenchmarkNodeChainTx1000__(b *testing.B) {
	nodeChainTx(b, 1000)
}

func BenchmarkNodeChainTx10000_(b *testing.B) {
	nodeChainTx(b, 10000)
}

/*
func BenchmarkNodeChainTx30000_(b *testing.B) {
	nodeChainTx(b, 30000)
}
*/

// nodeChain benchmarks the creating then querying a node chain.
func nodeChainTx(b *testing.B, chainLength int) {
	b.StopTimer()
	db := connectBench(b)
	defer benchCleanup(b, db)
	db.CreateIndex("Person", "name")
	b.StartTimer()
	for cnt := 0; cnt < b.N; cnt++ {
		qs := []*CypherQuery{}
		cq := CypherQuery{
			Statement:  `CREATE (n:Person {name: {i}}) RETURN n`,
			Parameters: Props{"i": 0},
			Result:     &[]Node{},
		}
		qs = append(qs, &cq)
		for i := 0; i < chainLength; i++ {
			cq := CypherQuery{
				Statement: `
					CREATE (a:Person {name: {a_name}})-[r:knows]->(b:Person {name: {b_name}})
					RETURN a.name, type(r), b.name
				`,
				Parameters: Props{
					"a_name": i,
					"b_name": i + 1,
				},
				Result: &[]struct {
					A int    `json:"a.name"`
					R string `json:"type(r)"`
					B int    `json:"b.name"`
				}{},
			}
			qs = append(qs, &cq)
		}
		res1 := []struct {
			A int    `json:"a.name"`
			R string `json:"type(r)"`
			B int    `json:"b.name"`
		}{}
		cq1 := CypherQuery{
			Statement: `
					MATCH (a:Person)-[r]->(b:Person)
					RETURN a.name, type(r), b.name
				`,
			Result: &res1,
		}
		qs = append(qs, &cq1)
		tx, err := db.Begin(qs)
		if err != nil {
			tx.Rollback()
			logPretty(err)
			b.Fatal(err)
		}
		err = tx.Commit()
		if err != nil {
			b.Fatal(err)
		}
		if len(res1) != chainLength {
			b.Fatal("Incorrect result length")
		}
		b.StopTimer()
		benchCleanup(b, db)
		b.StartTimer()
	}
}
