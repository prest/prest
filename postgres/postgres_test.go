package postgres

import (
	"net/http"
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

func TestWhereByRequest(t *testing.T) {
	Convey("Where by request", t, func() {
		r, err := http.NewRequest("GET", "/databases?dbname=prest&test=cool", nil)
		So(err, ShouldBeNil)
		where := WhereByRequest(r)
		So(where, ShouldContainSubstring, "dbname='prest'")
		So(where, ShouldContainSubstring, "test='cool'")
		So(where, ShouldContainSubstring, "and")
	})
}

func TestConnection(t *testing.T) {
	Convey("Verify database connection", t, func() {
		sqlx := Conn()
		So(sqlx, ShouldNotBeNil)
		err := sqlx.Ping()
		So(err, ShouldBeNil)
	})
}
