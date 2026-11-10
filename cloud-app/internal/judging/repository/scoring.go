package repository

import (
	"context"

	"kicau.cloud/control/internal/core/model"
)

func (r *Repository) Arena(ctx context.Context, id string) (model.Record, error) {
	return r.db.One(ctx, "EntityArenasGet", id)
}
func (r *Repository) LockArena(ctx context.Context, id string) (model.Record, error) {
	return r.db.One(ctx, "LockArena", id)
}
func (r *Repository) Round(ctx context.Context, id string) (model.Record, error) {
	return r.db.One(ctx, "EntityRoundsGet", id)
}
func (r *Repository) LockRound(ctx context.Context, id string) (model.Record, error) {
	return r.db.One(ctx, "LockRound", id)
}
func (r *Repository) Criteria(ctx context.Context, arena string) ([]model.Record, error) {
	return r.db.Many(ctx, "ArenaCriteria", arena)
}
func (r *Repository) Assignment(ctx context.Context, round, actor string) (model.Record, error) {
	return r.db.One(ctx, "JudgeAssignment", round, actor)
}
func (r *Repository) Rounds(ctx context.Context, actor, cursor string, limit int) ([]model.Record, error) {
	return r.db.Many(ctx, "JudgeRounds", actor, cursor, limit)
}
func (r *Repository) Item(ctx context.Context, id string) (model.Record, error) {
	return r.db.One(ctx, "JudgeItem", id)
}
func (r *Repository) Items(ctx context.Context, round string) ([]model.Record, error) {
	return r.db.Many(ctx, "RoundItems", round)
}
func (r *Repository) Assessment(ctx context.Context, item, judge string) (model.Record, error) {
	return r.db.One(ctx, "AssessmentGet", item, judge)
}
func (r *Repository) SaveAssessment(ctx context.Context, arena, round, item, judge string, version int) (model.Record, error) {
	return r.db.One(ctx, "AssessmentSave", arena, round, item, judge, version)
}
func (r *Repository) Scores(ctx context.Context, item, judge string) ([]model.Record, error) {
	return r.db.Many(ctx, "AssessmentScores", item, judge)
}
func (r *Repository) History(ctx context.Context, item, judge string) ([]model.Record, error) {
	return r.db.Many(ctx, "AssessmentHistory", item, judge)
}
func (r *Repository) Score(ctx context.Context, arena, round, item, judge, criterion string, value, version int) (model.Record, error) {
	return r.db.One(ctx, "ScoreUpsert", arena, round, item, judge, criterion, value, version)
}
func (r *Repository) Revision(ctx context.Context, id, operation string, version, value int) error {
	return r.db.Exec(ctx, "ScoreRevision", id, operation, version, value)
}
func (r *Repository) Operation(ctx context.Context, actor, operation string) (model.Record, error) {
	return r.db.One(ctx, "ScoreOperation", actor, operation)
}
func (r *Repository) SaveOperation(ctx context.Context, actor, operation string, hash []byte, assessment string, result string) error {
	return r.db.Exec(ctx, "ScoreOperationSave", actor, operation, hash, assessment, result)
}
