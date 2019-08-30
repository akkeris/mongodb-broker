package broker

import (
	"encoding/json"
	"errors"
	_ "github.com/lib/pq"
	"gopkg.in/mgo.v2"
	"net/url"
	"strings"
	"time"
)

var (
	Session       *mgo.Session
	BrokerDB      *mgo.Database
	plans         PlanSpec
	plansMap      map[string]string
	namePrefix    string
	engineversion string
)

const (
	brokerDbName        string = "broker"
	provisionCollection string = "provision"
	plansCollection     string = "plans"
)

type DatabaseSpec struct {
	Name        string    `json:"name"`
	Username    string    `json:"username"`
	Password    string    `json:"password"`
	Created     time.Time `json:"created"`
	Host        string    `json:"hostname"`
	Port        string    `json:"port"`
	Plan        string    `json:"plan"`
	BillingCode string    `json:"billingcode"`
	Misc        string    `json:"misc"`
}

type DBUrl struct {
	Url string `json:"MONGODB_URL"`
}

type FullDatabaseSpec struct {
	DatabaseSpec
	DBUrl
}

type InfoData struct {
	DatabaseName string
	BillingCode  string
	DATABASE_URL string
}

type PlanSpec struct {
	Name        string `json:"name"`
	Size        string `json:"size"`
	Description string `json:"description"`
}

type ProvisionSpec struct {
	Plan        string
	BillingCode string
	Misc        string
}

type MsgSpec struct {
	Msg string `json:"message"`
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

func (mpps MongodbProviderPlanSettings) GetMasterUriWithDb(dbName string) string {
	db, err := url.Parse(mpps.MasterUri)
	if err != nil {
		return ""
	}
	pass, ok := db.User.Password()
	if ok == true {
		return "mongodb://" + db.User.Username() + ":" + pass + "@" + db.Host + "/" + dbName + "?" + db.RawQuery
	} else if db.User.Username() != "" {
		return "mongodb://" + db.User.Username() + "@" + db.Host + "/" + dbName + "?" + db.RawQuery
	} else {
		return "mongodb://" + db.Host + "/" + dbName + "?" + db.RawQuery
	}
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
		return nil, err
	}

	pSession, err = mgo.Dial(settings.MasterUri)
	if err != nil {
		return err
	}

	pSession.SetMode(mgo.Monotonic, true)
	defer pSession.Close()

	pRoles := []mgo.Role{
		mgo.RoleReadWrite,
		mgo.RoleDBAdmin,
	}

	c := pSession.DB("broker").C("provision")

	pSpec := DatabaseSpec{}
	pSpec.Name = strings.ToLower(provider.namePrefix + RandomString(8))
	pSpec.Username = strings.ToLower("u" + RandomString(8))
	pSpec.Password = RandomString(16)
	pSpec.Created = time.Now()
	pSpec.Plan = plan.ID
	pSpec.BillingCode = Owner
	pSpec.Host = Dbc.DbHosts[0]
	pSpec.Port = Dbc.DbPort

	err = c.Insert(&pSpec)
	if err != nil {
		return nil, err
	} else {
		pUser := mgo.User{
			Username: pSpec.Username,
			Password: pSpec.Password,
			Roles:    pRoles,
			CustomData: InfoData{
				DatabaseName: pSpec.Name,
				BillingCode:  pSpec.BillingCode,
			},
		}

		err = pSession.DB(pSpec.Name).UpsertUser(&pUser)
		if err != nil {
			return nil, err
		}
	}

	return &Instance{
		Id:            Id,
		Name:          pSpec.Name,
		ProviderId:    pSpec.Name,
		Plan:          plan,
		Username:      pSpec.Username,
		Password:      pSpec.Password,
		Endpoint:      settings.MasterHost() + "/" + pSpec.Name,
		Status:        "available",
		Ready:         true,
		Engine:        settings.Engine,
		EngineVersion: settings.EngineVersion,
		Scheme:        plan.Scheme,
	}, nil
}

func (provider MongodbProvider) Deprovision(instance *Instance, takeSnapshot bool) error {
	var settings MongodbProviderPlanSettings
	if err := json.Unmarshal([]byte(Instance.Plan.providerPrivateDetails), &settings); err != nil {
		return err
	}

	rSession, err = mgo.Dial(settings.MasterUri)
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

		if err != nil {
			return err
		} else {
			err = rSession.DB("").C(provisionCollection).Remove(r)
			if err != nil {
				return nil
			}
		}
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

func plansInit() error {
	var err error

	pSession := BrokerDB.Session.Copy()
	defer pSession.Close()

	pColl := pSession.DB(brokerDbName).C(plansCollection)

	err = pColl.Find(nil).All(&plans)

	if err == nil {
		if len(plans) < 1 {
			plan := model.PlanSpec{
				Name:        "shared",
				Size:        "Unlimited",
				Description: "Shared Server",
			}
			plans = append(plans, plan)
			err = pColl.Insert(&plan)
			plan = model.PlanSpec{
				Name:        "ha",
				Size:        "100gb",
				Description: "High Availability",
			}
			plans = append(plans, plan)
			err = pColl.Insert(&plan)
		}
	}

	plansMap = map[string]string{}

	for _, p := range plans {
		plansMap[p.Name] = p.Description
	}
	return err
}
