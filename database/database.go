package database

import (
	"errors"
	"fmt"
	"strconv"

	"github.com/thebigear/utils"
	"gopkg.in/mgo.v2"
)

// Query represents a group of criteria to query database
type Query map[string]interface{}

// Index represents indexes
type Index map[string]interface{}

// QuerySlice represents a group of criteria to query database
type QuerySlice []map[string]interface{}

// DocumentChange represents a changes of document
type DocumentChange mgo.Change

// PaginationParams should used for passing pagination related parameters to models
type PaginationParams struct {
	Limit  int
	SortBy string
	Page   int
}

// GeoJSON should used to parse location data
type GeoJSON struct {
	Type        string    `json:"-"`
	Coordinates []float64 `json:"coordinates"`
}

// MongoConn holds session information of MongoDB connection
type MongoConn struct {
	Session  *mgo.Session
	DialInfo *mgo.DialInfo
}

// DateTimeLayout represents common layout of datetime across all interfaces of this app
const DateTimeLayout = "2006-01-02T15:04:05.000Z"

var (
	// ErrNotFound reflects model errors
	ErrNotFound = errors.New("not found")

	// Mongo represents connected db
	Mongo *MongoConn
)

// GeoJSONFromCoords returns a GeoJSON with given coordinates
func GeoJSONFromCoords(latitude, longitude float64) GeoJSON {
	return GeoJSON{
		Type:        "Point",
		Coordinates: []float64{longitude, latitude},
	}
}

// NewPaginationParams creates and returns new PaginationParams
func NewPaginationParams() *PaginationParams {
	return &PaginationParams{
		Limit:  50,
		SortBy: "-_id",
		Page:   0}
}

// PaginationParamsForContext creates and returns new PaginationParams for given context
func PaginationParamsForContext(pageQuery, limitQuery, sortByQuery string) *PaginationParams {
	limit, limitErr := strconv.Atoi(limitQuery)
	if limitErr != nil {
		limit = 7500
	}

	sortBy := "-_id"
	if sortByQuery != "" {
		sortBy = sortByQuery
	}

	page, pageErr := strconv.Atoi(pageQuery)
	if pageErr != nil {
		page = 0
	}

	return &PaginationParams{
		Limit:  limit,
		SortBy: sortBy,
		Page:   page}
}

// Connect connects to mongodb
func Connect() {
	uri := utils.GetEnvOrDefault("MONGO_URL", "mongodb://localhost:27017/thebigear")
	mongo, err := mgo.ParseURL(uri)
	s, err := mgo.Dial(uri)
	if err != nil {
		fmt.Printf("Can't connect to mongo, go error %v\n", err)
		panic(err.Error())
	}
	s.SetSafe(&mgo.Safe{})
	fmt.Println("Connected to", uri)

	Mongo = &MongoConn{
		Session:  s,
		DialInfo: mongo,
	}
}

// EnsureIndexes ensure indexes
func EnsureIndexes() {
	index := mgo.Index{
		Key:        []string{"$2dsphere:geojson"},
		Unique:     false,
		DropDups:   false,
		Background: true,
		Sparse:     true,
	}
	Mongo.EnsureIndex("expressions", index)
}

// // CloneSession provides echo MiddlewareFunc that clones session for each request
// func CloneSession() echo.MiddlewareFunc {
// 	return func(h echo.HandlerFunc) echo.HandlerFunc {
// 		return func(c echo.Context) error {
// 			// s := Mongo.Session.Clone()
// 			// defer s.Close()
//
// 			// c.Set("db", Mongo.Session.DB(Mongo.DialInfo.Database))
// 			return nil
// 		}
// 	}
// }

// FindOne returns first object with matching criteria
func (db *MongoConn) FindOne(collection string, query Query, result interface{}) error {
	return db.Session.
		DB(db.DialInfo.Database).
		C(collection).
		Find(query).
		One(result)
}

// FindAll returns all documents matching with criteria
func (db *MongoConn) FindAll(collection string, query Query, result interface{}, pagination *PaginationParams) error {
	queryResult := db.Session.
		DB(db.DialInfo.Database).
		C(collection).
		Find(query)

	if pagination != nil {
		queryResult = queryResult.
			Sort(pagination.SortBy).
			Skip(pagination.Page * pagination.Limit).
			Limit(pagination.Limit)
	}

	return queryResult.All(result)
}

// FindLast returns last object with matching criteria
func (db *MongoConn) FindLast(collection string, query Query, result interface{}) error {
	return db.Session.
		DB(db.DialInfo.Database).
		C(collection).
		Find(query).
		Sort("-_id").
		One(result)
}

// FindAllGeo returns all documents matching with criteria
func (db *MongoConn) FindAllGeo(collection string, query Query, result interface{}, pagination *PaginationParams) error {
	queryResult := db.Session.
		DB(db.DialInfo.Database).
		C(collection).
		Find(query)

	if pagination != nil {
		queryResult = queryResult.
			Skip(pagination.Page * pagination.Limit).
			Limit(pagination.Limit)
	}

	return queryResult.All(result)
}

// Count returns document count of given query
func (db *MongoConn) Count(collection string, query Query) (int, error) {
	return db.Session.
		DB(db.DialInfo.Database).
		C(collection).
		Find(query).
		Count()
}

// Aggregate returns document aggregate look at pipe
func (db *MongoConn) Aggregate(collection string, query QuerySlice, result interface{}) error {
	return db.Session.
		DB(db.DialInfo.Database).
		C(collection).
		Pipe(query).
		All(result)
}

// Distinct returns document distinct by given field
func (db *MongoConn) Distinct(collection string, distinctField string, query Query, result interface{}) error {
	return db.Session.
		DB(db.DialInfo.Database).
		C(collection).
		Find(query).
		Distinct(distinctField, result)
}

// Insert creates new object
func (db *MongoConn) Insert(collection string, obj interface{}) error {
	return db.Session.
		DB(db.DialInfo.Database).
		C(collection).
		Insert(obj)
}

// Update updates and returns document with given parameters
func (db *MongoConn) Update(collection string, query Query, change DocumentChange, result interface{}) error {
	_, err := db.Session.
		DB(db.DialInfo.Database).
		C(collection).
		Find(query).
		Apply(mgo.Change(change), result)

	return err
}

// UpdateAll updates and returns all documents matching with given parameters
func (db *MongoConn) UpdateAll(collection string, query Query, change Query) (int, error) {
	changeInfo, err := db.Session.
		DB(db.DialInfo.Database).
		C(collection).
		UpdateAll(query, change)

	if changeInfo == nil {
		return 0, err
	}

	return changeInfo.Updated, err
}

// RemoveOne removes document with given criteria
func (db *MongoConn) RemoveOne(collection string, query Query) error {
	return db.Session.
		DB(db.DialInfo.Database).
		C(collection).
		Remove(query)
}

// RemoveAll removes all documents with given criteria
func (db *MongoConn) RemoveAll(collection string, query Query) error {
	_, err := db.Session.
		DB(db.DialInfo.Database).
		C(collection).
		RemoveAll(query)

	return err
}

// DropCollection drops collection
func (db *MongoConn) DropCollection(collection string) error {
	return db.Session.
		DB(db.DialInfo.Database).
		C(collection).
		DropCollection()
}

// Exists checks if document exists with given criteria
func (db *MongoConn) Exists(collection string, query Query) bool {
	var result interface{}

	err := db.Session.
		DB(db.DialInfo.Database).
		C(collection).
		Find(query).
		One(result)

	return (err == nil)
}

// DropDatabase drops database
func (db *MongoConn) DropDatabase() error {
	return db.Session.
		DB(db.DialInfo.Database).
		DropDatabase()
}

// EnsureIndex ensures index
func (db *MongoConn) EnsureIndex(collection string, index mgo.Index) error {
	return db.Session.
		DB(db.DialInfo.Database).
		C(collection).
		EnsureIndex(index)
}
