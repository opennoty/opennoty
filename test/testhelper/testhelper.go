package testhelper

import (
	"context"
	"encoding/json"
	mim "github.com/ONSdigital/dp-mongodb-in-memory"
	"github.com/spf13/afero"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"log"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"
	_ "unsafe"
)

//go:linkname afs github.com/ONSdigital/dp-mongodb-in-memory/download.afs
var afs afero.Afero

type TestEnv struct {
	setup bool

	timeout time.Duration

	Ctx    context.Context
	cancel func()

	MongoServer *mim.Server

	mongoClient   *mongo.Client
	mongoDatabase *mongo.Database

	collections map[string]*mongo.Collection
}

var testEnv TestEnv

func init() {
	testEnv.collections = map[string]*mongo.Collection{}
}

func Setup() {
	var err error

	if testEnv.setup {
		return
	}
	testEnv.setup = true

	afs = afero.Afero{Fs: NewOsFs()}

	testEnv.Ctx, testEnv.cancel = context.WithCancel(context.Background())

	testEnv.MongoServer, err = mim.StartWithReplicaSet(testEnv.Ctx, "5.0.2", "rs0")
	if err != nil {
		panic(err)
	}
}

func Cleanup() {
	if testEnv.MongoServer != nil {
		testEnv.MongoServer.Stop(testEnv.Ctx)
	}
	if testEnv.cancel != nil {
		testEnv.cancel()
	}
}

func SetTimeout(timeout time.Duration) {
	testEnv.timeout = timeout
}

func TestMain(m *testing.M) {
	var code int
	Setup()
	if testEnv.timeout != 0 {
		go func() {
			select {
			case <-time.After(testEnv.timeout):
				testEnv.cancel()
				log.Println("TEST TIMED OUT")
			case <-testEnv.Ctx.Done():
				break
			}
		}()
	}
	code = m.Run()
	defer func() {
		Cleanup()
		os.Exit(code)
	}()
}

func GetTestEnv() *TestEnv {
	if !testEnv.setup {
		Setup()
	}
	return &testEnv
}

func (e *TestEnv) GetMongoUri() string {
	mongoUri, err := url.Parse(e.MongoServer.URI())
	if err != nil {
		panic(err)
	}
	mongoUri = mongoUri.JoinPath("test")
	return mongoUri.String()
}

func (e *TestEnv) GetMongoDatabase() *mongo.Database {
	if e.mongoDatabase != nil {
		return e.mongoDatabase
	}

	mongoUri, err := url.Parse(e.GetMongoUri())
	if err != nil {
		panic(err)
	}
	dbName := strings.TrimPrefix(mongoUri.Path, "/")

	mongoOptions := options.Client().ApplyURI(mongoUri.String())
	e.mongoClient, err = mongo.Connect(context.TODO(), mongoOptions)
	if err != nil {
		panic(err)
	}

	e.mongoDatabase = e.mongoClient.Database(dbName)

	return e.mongoDatabase
}

func (e *TestEnv) InsertDocument(collectionName string, doc interface{}) {
	e.InsertDocuments(collectionName, []interface{}{doc})
}

func (e *TestEnv) InsertDocuments(collectionName string, documents []interface{}) {
	database := e.GetMongoDatabase()
	collection, ok := e.collections[collectionName]
	if !ok {
		collection = database.Collection(collectionName)
		e.collections[collectionName] = collection
	}
	res, err := collection.InsertMany(e.Ctx, documents)
	if err != nil {
		panic(err)
	}
	_ = res
}

func InsertDocumentsFromJson[D interface{}](collectionName string, jsonRaw []byte) {
	var documents []D
	if err := json.Unmarshal(jsonRaw, &documents); err != nil {
		panic(err)
	}
	testEnv := GetTestEnv()

	var temp []interface{}
	for _, document := range documents {
		temp = append(temp, document)
	}
	testEnv.InsertDocuments(collectionName, temp)
}
