package connection

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

func TestParse(t *testing.T) {
	Convey("Check connection", t, func() {
		db := MustGet()
		So(db, ShouldNotBeNil)
		err := db.Ping()
		So(err, ShouldBeNil)
	})
}
