package broker

import (
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	_ "github.com/lib/pq"
	"gopkg.in/mgo.v2"
	"net"
	"net/url"
	"os"
	"strings"
)

type InfoData struct {
	DatabaseName string
	BillingCode  string
	DATABASE_URL string
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
		Endpoint:      settings.MasterHost() + "/" + name,
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

func (provider MongodbProvider) Provision(Id string, plan *ProviderPlan, Owner string) (*Instance, error) {
	var settings MongodbProviderPlanSettings
	if err := json.Unmarshal([]byte(plan.providerPrivateDetails), &settings); err != nil {
		fmt.Println(err)
		return nil, err
	}

	fmt.Println(settings)

	dialInfo, err := mgo.ParseURL(settings.MasterUri)
	dialInfo.DialServer = func(addr *mgo.ServerAddr) (net.Conn, error) {
		conn, err := tls.Dial("tcp", addr.String(), nil)
		return conn, err
	}

	fmt.Println(dialInfo)
	if err != nil {
		fmt.Println("Failed to parse URI: ", err)
		os.Exit(1)
	}

	pSession, err := mgo.DialWithInfo(dialInfo)
	if err != nil {
		fmt.Println(err)
		return nil, err
	}

	pSession.SetMode(mgo.Monotonic, true)
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

		err = pSession.DB(name).UpsertUser(&pUser)
		if err != nil {
			fmt.Println(err)
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
		Endpoint:      settings.MasterHost() + "/" + name,
		Status:        "available",
		Ready:         true,
		Engine:        settings.Engine,
		EngineVersion: settings.EngineVersion,
		Scheme:        plan.Scheme,
	}, nil
}

func (provider MongodbProvider) Deprovision(instance *Instance, takeSnapshot bool) error {
	var settings MongodbProviderPlanSettings
	if err := json.Unmarshal([]byte(instance.Plan.providerPrivateDetails), &settings); err != nil {
		return err
	}
	dialInfo, err := mgo.ParseURL(settings.MasterUri)
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
		return err
	}

	rSession.SetMode(mgo.Monotonic, true)
	defer rSession.Close()

	if err != nil {
		return err
	} else {
		err = rSession.DB(instance.Name).RemoveUser(instance.Username)
		if err != nil {
			return err
		}

		err = rSession.DB(instance.Name).DropDatabase()
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
