package broker

import (
	"context"
	"crypto/tls"
	"fmt"
	_ "github.com/lib/pq"
	osb "github.com/pmorie/go-open-service-broker-client/v2"
	"github.com/pmorie/osb-broker-lib/pkg/broker"
	. "github.com/smartystreets/goconvey/convey"
	"gopkg.in/mgo.v2"
	"net"
	"os"
	"testing"
)

func TestPostgresProvision(t *testing.T) {
	var logic *BusinessLogic
	var catalog *broker.CatalogResponse
	var plan osb.Plan
	var dbUrl string
	var instanceId string = RandomString(12)
	var err error

	Convey("Given a fresh provisioner.", t, func() {
		So(os.Getenv("DATABASE_URL"), ShouldNotEqual, "")
		So(os.Getenv("MONGO_DB_SHARED_URI"), ShouldNotEqual, "")
		logic, err = NewBusinessLogic(context.TODO(), Options{DatabaseUrl: os.Getenv("DATABASE_URL"), NamePrefix: "test"})
		So(err, ShouldBeNil)
		So(logic, ShouldNotBeNil)

		Convey("Ensure we can get the catalog and target plan exists", func() {
			rc := broker.RequestContext{}
			catalog, err = logic.GetCatalog(&rc)
			So(err, ShouldBeNil)
			So(catalog, ShouldNotBeNil)
			So(len(catalog.Services), ShouldEqual, 1)

			var shared = false
			for _, p := range catalog.Services[0].Plans {
				if p.Name == "shared" {
					plan = p
					shared = true
				}
			}
			So(shared, ShouldEqual, true)

			var ha = false
			for _, p := range catalog.Services[0].Plans {
				if p.Name == "high-availability" {
					ha = true
				}
			}
			So(ha, ShouldEqual, true)
		})

		Convey("Ensure provisioner for mongodb can provision a database", func() {
			var request osb.ProvisionRequest
			var c broker.RequestContext
			request.AcceptsIncomplete = false
			res, err := logic.Provision(&request, &c)
			So(res, ShouldBeNil)
			So(err, ShouldNotBeNil)
			So(err.Error(), ShouldEqual, "Status: 422; ErrorMessage: <nil>; Description: The query parameter accepts_incomplete=true MUST be included the request.; ResponseError: AsyncRequired")

			request.AcceptsIncomplete = true
			request.PlanID = "does not exist"
			request.InstanceID = "asfdasdf"
			res, err = logic.Provision(&request, &c)
			So(err.Error(), ShouldEqual, "Status: 404; ErrorMessage: <nil>; Description: Not Found; ResponseError: <nil>")

			request.InstanceID = instanceId
			request.PlanID = plan.ID
			res, err = logic.Provision(&request, &c)

			So(err, ShouldBeNil)
			So(res, ShouldNotBeNil)
		})

		Convey("Get and create service bindings", func() {
			var request osb.LastOperationRequest = osb.LastOperationRequest{InstanceID: instanceId}
			var c broker.RequestContext
			res, err := logic.LastOperation(&request, &c)
			So(err, ShouldBeNil)
			So(res, ShouldNotBeNil)
			So(res.State, ShouldEqual, osb.StateSucceeded)

			var guid = "123e4567-e89b-12d3-a456-426655440000"
			var resource osb.BindResource = osb.BindResource{AppGUID: &guid}
			var brequest osb.BindRequest = osb.BindRequest{InstanceID: instanceId, BindingID: "foo", BindResource: &resource}
			dres, err := logic.Bind(&brequest, &c)
			So(err, ShouldBeNil)
			So(dres, ShouldNotBeNil)
			So(dres.Credentials["DATABASE_URL"].(string), ShouldStartWith, "mongodb://")

			dbUrl = dres.Credentials["DATABASE_URL"].(string)

			var gbrequest osb.GetBindingRequest = osb.GetBindingRequest{InstanceID: instanceId, BindingID: "foo"}
			gbres, err := logic.GetBinding(&gbrequest, &c)
			So(err, ShouldBeNil)
			So(gbres, ShouldNotBeNil)
			So(gbres.Credentials["DATABASE_URL"].(string), ShouldStartWith, "mongodb://")
			So(gbres.Credentials["DATABASE_URL"].(string), ShouldStartWith, dres.Credentials["DATABASE_URL"].(string))
		})

		Convey("Connecting to MongoDB", func() {
			dialInfo, err := mgo.ParseURL(dbUrl)
			dialInfo.DialServer = func(addr *mgo.ServerAddr) (net.Conn, error) {
				conn, err := tls.Dial("tcp", addr.String(), nil)
				return conn, err
			}
			if err != nil {
				fmt.Println("Failed to parse URI: ", err)
				os.Exit(1)
			}
			rSession, err := mgo.DialWithInfo(dialInfo)
			if err != nil {
				fmt.Println(err.Error())
			}
			So(err, ShouldBeNil)
			rSession.SetMode(mgo.Monotonic, true)
			defer rSession.Close()

			db := rSession.DB("")
			So(db.Name, ShouldContainSubstring, "test")
			colls, err := db.CollectionNames()
			So(err, ShouldBeNil)
			So(colls, ShouldBeEmpty)

			b, err := rSession.BuildInfo()
			if err != nil {
				fmt.Println(err.Error())
			}
			So(err, ShouldBeNil)
			So(b.Version, ShouldNotBeBlank)

		})

		Convey("Ensure unbind for mongodb works", func() {
			var c broker.RequestContext
			var urequest osb.UnbindRequest = osb.UnbindRequest{InstanceID: instanceId, BindingID: "foo"}
			ures, err := logic.Unbind(&urequest, &c)
			So(err, ShouldBeNil)
			So(ures, ShouldNotBeNil)
		})

		Convey("Ensure deprovisioner for mongodb works", func() {
			var request osb.LastOperationRequest = osb.LastOperationRequest{InstanceID: instanceId}
			var c broker.RequestContext
			res, err := logic.LastOperation(&request, &c)
			So(err, ShouldBeNil)
			So(res, ShouldNotBeNil)
			So(res.State, ShouldEqual, osb.StateSucceeded)

			var drequest osb.DeprovisionRequest = osb.DeprovisionRequest{InstanceID: instanceId}
			dres, err := logic.Deprovision(&drequest, &c)

			So(err, ShouldBeNil)
			So(dres, ShouldNotBeNil)
		})
	})
}
