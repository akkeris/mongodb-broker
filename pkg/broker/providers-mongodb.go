package broker

import (
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/golang/glog"
	_ "github.com/lib/pq"

	// "gopkg.in/mgo.v2"
	"net"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/globalsign/mgo"
)

type InfoData struct {
	DatabaseName string
	BillingCode  string
	MONGODB_URL  string
}

// provider=mongodb in database
// These values come out of the plans table provider_private_details column.
type MongodbProviderPlanSettings struct {
	MasterUri     string `json:"master_uri"`
	Engine        string `json:"engine"`
	EngineVersion string `json:"engine_version"`
}

func (mpps MongodbProviderPlanSettings) MasterHost() string {
	db, err := url.Parse(mpps.MasterUri)
	if err != nil {
		return ""
	}
	return db.Host
}

type MongodbProvider struct {
	Provider
	namePrefix string
}

func NewMongodbProvider(namePrefix string) (MongodbProvider, error) {
	return MongodbProvider{
		namePrefix: namePrefix,
	}, nil
}

func (provider MongodbProvider) GetInstance(name string, plan *ProviderPlan) (*Instance, error) {
	var settings MongodbProviderPlanSettings

	glog.V(3).Infof("[p.GetInstance] start name: %s, plan: %s", name, plan.ID)

	if err := json.Unmarshal([]byte(plan.providerPrivateDetails), &settings); err != nil {
		return nil, err
	}

	return &Instance{
		Id:            "", // provider should not store this.
		Name:          name,
		ProviderId:    name,
		Plan:          plan,
		Username:      "", // provider should not store this.
		Password:      "", // provider should not store this.
		Endpoint:      settings.MasterHost() + "/" + name + "?ssl=true",
		Status:        "available",
		Ready:         true,
		Engine:        "mongodb",
		EngineVersion: settings.EngineVersion,
		Scheme:        "mongodb",
	}, nil
}

func (provider MongodbProvider) PerformPostProvision(db *Instance) (*Instance, error) {
	return db, nil
}

func redactMongoDbURL(dburl string) string {
	mInfo, err := mgo.ParseURL(dburl)

	if err != nil {
		return dburl
	}

	return fmt.Sprintf("mongodb://[REDACTED]:[REDACTED]@%s/%s?authSource=%s", mInfo.Addrs[0], mInfo.Database, mInfo.Source)
}

func connectToMongoDb(mongoDbUri string) (*mgo.Session, error) {
	glog.V(3).Infoln("[connectToMongoDb] start")

	dialInfo, err := mgo.ParseURL(mongoDbUri)

	dialInfo.Timeout = time.Second * 30
	dialInfo.Direct = true
	dialInfo.FailFast = true
	dialInfo.DialServer = func(addr *mgo.ServerAddr) (net.Conn, error) {
		return tls.Dial("tcp", addr.String(), nil)
	}

	if err != nil {
		fmt.Println("Failed to parse master mongodb URI: ", err)
		os.Exit(1)
	}

	glog.V(1).Infof("[m.connectToMongoDb] connect to mongodb: %s\n", redactMongoDbURL(mongoDbUri))

	glog.V(4).Infof("[m.connectToMongoDb] dialInfo: %+v", dialInfo)

	pSession, err := mgo.DialWithInfo(dialInfo)
	if err != nil {
		glog.Errorf("[m.connectToMongoDb] error: %s", err)
		return nil, err
	}

	glog.V(3).Infoln("[m.connectToMongoDb] SetMode")

	pSession.SetMode(mgo.Monotonic, true)

	return pSession, nil
}

func (provider MongodbProvider) Provision(Id string, plan *ProviderPlan, Owner string) (*Instance, error) {
	var settings MongodbProviderPlanSettings

	glog.Infof("[m.Provision] start id: %s, plan %s\n", Id, plan.ID)
	glog.V(4).Infof("[m.Provision] private details: %+v", plan.providerPrivateDetails)

	if err := json.Unmarshal([]byte(plan.providerPrivateDetails), &settings); err != nil {
		fmt.Println(err)
		return nil, err
	}

	glog.V(3).Infof("[m.Provision] plan settings: %+v", settings)

	pSession, err := connectToMongoDb(settings.MasterUri)
	if err != nil {
		return nil, err
	}
	defer pSession.Close()

	pRoles := []mgo.Role{
		mgo.RoleReadWrite,
		mgo.RoleDBAdmin,
	}

	var name = strings.ToLower(provider.namePrefix + RandomString(8))
	var username = strings.ToLower("u" + RandomString(8))
	var password = RandomString(16)
	var billingcode = Owner

	if err != nil {
		fmt.Println(err)
		return nil, err
	} else {
		pUser := mgo.User{
			Username: username,
			Password: password,
			Roles:    pRoles,
			CustomData: InfoData{
				DatabaseName: name,
				BillingCode:  billingcode,
			},
		}

		glog.V(3).Infof("[m.Provision] Upsert user: %s\n", pUser.Username)

		err = pSession.DB(name).UpsertUser(&pUser)
		if err != nil {
			glog.V(3).Info(err)
			return nil, err
		}
	}

	return &Instance{
		Id:            Id,
		Name:          name,
		ProviderId:    name,
		Plan:          plan,
		Username:      username,
		Password:      password,
		Endpoint:      settings.MasterHost() + "/" + name + "?ssl=true",
		Status:        "available",
		Ready:         true,
		Engine:        settings.Engine,
		EngineVersion: settings.EngineVersion,
		Scheme:        plan.Scheme,
	}, nil
}

func (provider MongodbProvider) Deprovision(instance *Instance, takeSnapshot bool) error {
	var settings MongodbProviderPlanSettings

	glog.V(3).Infof("[m.Deprovision] start instance: %s\n", instance.Id)

	if err := json.Unmarshal([]byte(instance.Plan.providerPrivateDetails), &settings); err != nil {
		return err
	}

	rSession, err := connectToMongoDb(settings.MasterUri)
	if err != nil {
		return err
	}
	defer rSession.Close()

	err = rSession.DB(instance.Name).RemoveUser(instance.Username)
	if err != nil {
		glog.Errorf("error removing user: %s", instance.Username)
		return err
	}

	err = rSession.DB(instance.Name).DropDatabase()
	if err != nil {
		glog.Errorf("error dropping: %s", instance.Name)
		return err
	}

	return err
}

func (provider MongodbProvider) Modify(instance *Instance, plan *ProviderPlan) (*Instance, error) {
	return nil,
		errors.New("This feature is not available on this plan.")
}

func (provider MongodbProvider) Tag(Instance *Instance, Name string, Value string) error {
	// do nothing
	return nil
}

func (provider MongodbProvider) Untag(Instance *Instance, Name string) error {
	// do nothing
	return nil
}
