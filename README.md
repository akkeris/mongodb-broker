# Akkeris MongoDB Broker

An Mongodb Broker for Akkeris.

## Providers

The broker has support for the following providers

* Shared MongoDB

## Installing

1. Create a postgres database
2. Create a mongodb database
3. Deploy the docker image (both worker and api) `akkeris/mongodb-broker:latest` run `start.sh` for the api, `start-backgrouund.sh` for worker. Both should use the same settigs (env) below.

### 1. Settings

Note almost all of these can be set via the command line as well.

**Required**

* `DATABASE_URL` - The postgres database to store its information on what databases its provisioned, this should be in the format of `postgres://user:password@host:port/database?sslmode=disable` or leave off sslmode=disable if ssl is supported.  This will auto create the schema if its unavailable.
* `NAME_PREFIX` - The prefix to use for all provisioned databases this should be short and help namespace databases created by the broker vs. other databases that may exist in the broker for other purposes. This is global to all of the providers configured.


**Optional**

* `PORT` - This defaults to 8443, setting this changes the default port number to listen to http (or https) traffic on
* `RETRY_WEBHOOKS` - (WORKER ONLY) whether outbound notifications about provisions or create bindings should be retried if they fail.  This by default is false, unless you trust or know the clients hitting this broker, leave this disabled.

### 2. Deployment

You can deploy the image `akkeris/mongodb-broker:latest` via docker with the environment or config var settings above. If you decide you're going to build this manually and run it you'll need see the Building section below. 

### 3. Plans

The plans table can be modified to adjust plans, at the moment only two exist, versioned and un-versioned. The default plans can be modified to make them unencrypted.

### 4. Setup Task Worker

You'll need to deploy one or multiple (depending on your load) task workers with the same config or settings specified in Step 1. but with a different startup command, append the `-background-tasks` option to the service brokers startup command to put it into worker mode.  You MUST have at least 1 worker.

## Running

As described in the setup instructions you should have two deployments for your application, the first is the API that receives requests, the other is the tasks process.  See `start.sh` for the API startup command, see `start-background.sh` for the tasks process startup command. Both of these need the above environment variables in order to run correctly.

**Debugging**

You can optionally pass in the startup options `-logtostderr=1 -stderrthreshold 0` to enable debugging, in addition you can set `GLOG_logtostderr=1` to debug via the environment.  See glog for more information on enabling various levels. You can also set `STACKIMPACT` as an environment variable to have profiling information sent to stack impact. 

## Contributing and Building

1. `export GO111MODULE=on`
2. `make`
3. `./servicebroker ...`

### Testing

Working on it...


