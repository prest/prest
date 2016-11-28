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
		So(where, ShouldEqual, "dbname='prest' and test='cool'")
	})
}
