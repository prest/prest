package connection

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

func TestMustGet(t *testing.T) {
	Convey("Check connection", t, func() {
		db := MustGet()
		So(db, ShouldNotBeNil)
		err := db.Ping()
		So(err, ShouldBeNil)
	})
}
