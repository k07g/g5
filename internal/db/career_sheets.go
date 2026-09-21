package db

import (
	"context"
	"errors"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"github.com/k07g/g5/internal/models"
)

var ErrCareerSheetNotFound = errors.New("career sheet not found")

const careerSheetsCollection = "career_sheets"

// careerSheetDocument is how a career sheet is stored: one document per
// user, keyed by the caller's Cognito sub as the MongoDB _id (so the id
// itself guarantees per-user uniqueness, with no separate index needed).
type careerSheetDocument struct {
	Data      models.CareerSheet `bson:"data"`
	CreatedAt time.Time          `bson:"created_at"`
	UpdatedAt time.Time          `bson:"updated_at"`
}

type CareerSheetRepository struct {
	collection *mongo.Collection
}

func NewCareerSheetRepository(database *mongo.Database) *CareerSheetRepository {
	return &CareerSheetRepository{collection: database.Collection(careerSheetsCollection)}
}

// Get returns the career sheet belonging to cognitoSub, or
// ErrCareerSheetNotFound if none has been saved yet.
func (r *CareerSheetRepository) Get(ctx context.Context, cognitoSub string) (*models.CareerSheet, error) {
	var doc careerSheetDocument
	err := r.collection.FindOne(ctx, bson.M{"_id": cognitoSub}).Decode(&doc)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return nil, ErrCareerSheetNotFound
	}
	if err != nil {
		return nil, err
	}

	doc.Data.Normalize()
	return &doc.Data, nil
}

// Upsert replaces the entire career sheet for cognitoSub, creating it if it
// doesn't exist yet. The frontend always sends the full document (it has
// no partial-update UI), so a whole-document replace is all this needs.
// UpdateOne with $set/$setOnInsert is used (rather than ReplaceOne) so
// created_at is preserved across updates instead of being reset every save.
func (r *CareerSheetRepository) Upsert(ctx context.Context, cognitoSub string, sheet *models.CareerSheet) error {
	now := time.Now()
	update := bson.M{
		"$set": bson.M{
			"data":       sheet,
			"updated_at": now,
		},
		"$setOnInsert": bson.M{
			"created_at": now,
		},
	}

	_, err := r.collection.UpdateOne(ctx, bson.M{"_id": cognitoSub}, update, options.UpdateOne().SetUpsert(true))
	return err
}

// Delete removes the career sheet for cognitoSub, if any.
func (r *CareerSheetRepository) Delete(ctx context.Context, cognitoSub string) error {
	res, err := r.collection.DeleteOne(ctx, bson.M{"_id": cognitoSub})
	if err != nil {
		return err
	}
	if res.DeletedCount == 0 {
		return ErrCareerSheetNotFound
	}
	return nil
}
