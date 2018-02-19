package models

import (
	"time"

	"github.com/rs/xid"
	"github.com/thebigear/database"
	"github.com/tuvistavie/structomap"
	"gopkg.in/mgo.v2/bson"
)

// DBTableExpressions collection name
const DBTableExpressions = "expressions"

// Expression structure
type Expression struct {
	ID                 bson.ObjectId `json:"-" bson:"_id,omitempty"`
	URLToken           string        `json:"-" bson:"token,omitempty"`
	FullText           string        `json:"full_text" bson:"full_text,omitempty"`
	CleanText          string        `json:"clean_text" bson:"clean_text,omitempty"`
	IsVerified         bool          `json:"is_verified,omitempty" bson:"is_verified,omitempty"`
	HasAttachment      bool          `json:"has_attachment,omitempty" bson:"has_attachment,omitempty"`
	Owner              string        `json:"owner,omitempty" bson:"owner,omitempty"`
	AttachmentLabels   string        `json:"attachment_labels,omitempty" bson:"attachment_labels,omitempty"`
	MediaURL           string        `json:"media_url,omitempty" bson:"media_url,omitempty"`
	Followers          int           `json:"followers,omitempty" bson:"followers,omitempty"`
	Following          int           `json:"following,omitempty" bson:"following,omitempty"`
	PostCount          int           `json:"post_count,omitempty" bson:"post_count,omitempty"`
	LastTenInteraction int           `json:"last_ten_interaction,omitempty" bson:"last_ten_interaction,omitempty"`
	TotalInteraction   int           `json:"total_interaction,omitempty" bson:"total_interaction,omitempty"`
	//Analysis  Analysis      `json:"analysis,omitempty" bson:"analysis,omitempty"`
	CreatedAt time.Time `json:"-" bson:"created_at,omitempty"`
	UpdatedAt time.Time `json:"-" bson:"updated_at,omitempty"`
	DeletedAt time.Time `json:"-" bson:"deleted_at,omitempty"`
}

// Expressions array representation of Expression
type Expressions []Expression

// ListExpressions lists all expressions
func ListExpressions(query database.Query, paginationParams *database.PaginationParams) (*Expressions, error) {
	var result Expressions

	if paginationParams == nil {
		paginationParams = database.NewPaginationParams()
		paginationParams.SortBy = "created_at"
	} else if paginationParams.SortBy == "-_id" {
		paginationParams.SortBy = "created_at"
	}

	err := database.Mongo.FindAll(DBTableExpressions, query, &result, paginationParams)
	if err != nil {
		return nil, err
	}
	return &result, nil
}

// GetExpression an expression title with token
func GetExpression(query database.Query) (*Expression, error) {
	var result Expression

	err := database.Mongo.FindOne(DBTableExpressions, query, &result)
	if err != nil {
		return nil, err
	}
	return &result, nil
}

// Create a new expression
func (expression *Expression) Create() (*Expression, error) {
	// TODO: Check against duplicate

	expression.URLToken = xid.New().String()
	expression.CreatedAt = time.Now()
	expression.UpdatedAt = expression.CreatedAt

	if err := database.Mongo.Insert(DBTableExpressions, expression); err != nil {
		return nil, err
	}

	return expression, nil
}

// Update an expression
func (expression *Expression) Update() (*Expression, error) {
	query := database.Query{}
	query["token"] = expression.URLToken

	expression.UpdatedAt = time.Now()

	change := database.DocumentChange{
		Update:    expression,
		ReturnNew: true,
	}

	result := &Expression{}
	err := database.Mongo.Update(DBTableExpressions, query, change, result)

	return result, err
}

// Delete an expression
func (expression *Expression) Delete() error {
	query := database.Query{}
	query["token"] = expression.URLToken

	expression.DeletedAt = time.Now()

	change := database.DocumentChange{
		Update:    expression,
		ReturnNew: true,
	}

	err := database.Mongo.Update(DBTableExpressions, query, change, nil)

	return err
}

// ExpressionSerializer used in constructing maps to output JSON
type ExpressionSerializer struct {
	*structomap.Base
}

// NewExpressionSerializer creates a new ExpressionSerializer
func NewExpressionSerializer() *ExpressionSerializer {
	s := &ExpressionSerializer{structomap.New()}
	s.Pick("RawText", "CleanText", "Source", "Image", "Owner", "Positive", "Polarity").
		PickFunc(func(t interface{}) interface{} {
			return t.(time.Time).Format(time.RFC3339)
		}, "CreatedAt", "UpdatedAt").
		AddFunc("ID", func(expression interface{}) interface{} {
			return expression.(Expression).URLToken
		})

	return s
}

// WithDeletedAt includes deletedAt field
func (s *ExpressionSerializer) WithDeletedAt() *ExpressionSerializer {
	s.PickFunc(func(t interface{}) interface{} {
		empty := time.Time{}
		if t.(time.Time) == empty {
			return nil
		}
		return t.(time.Time).Format(time.RFC3339)
	}, "DeletedAt")

	return s
}
