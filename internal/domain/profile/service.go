package profile

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/go-playground/validator/v10"
	"github.com/google/uuid"
	"github.com/microcosm-cc/bluemonday"
)

// Service orchestrates profile logic.
type Service struct {
	repo      Repository
	validator *validator.Validate
	sanitizer *bluemonday.Policy
}

// Sentinel errors for HTTP mapping.
var (
	ErrNotFound        = errors.New("profile not found")
	ErrForbidden       = errors.New("forbidden")
	ErrVersionConflict = errors.New("version mismatch")
)

// NewService provides a profile service.
func NewService(repo Repository) *Service {
	return &Service{
		repo:      repo,
		validator: validator.New(),
		sanitizer: bluemonday.UGCPolicy(),
	}
}

// Create persists a profile.
func (s *Service) Create(ctx context.Context, userID uuid.UUID, req CreateRequest) (*Profile, error) {
	if err := s.validator.Struct(req); err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	profile := &Profile{
		ID:        uuid.New(),
		UserID:    userID,
		FirstName: sanitizeField(s.sanitizer, req.FirstName),
		LastName:  sanitizeField(s.sanitizer, req.LastName),
		CreatedAt: now,
		UpdatedAt: now,
		Version:   1,
	}
	if err := assignOptionalFields(s.sanitizer, profile, req); err != nil {
		return nil, err
	}
	if err := s.repo.Create(ctx, profile); err != nil {
		return nil, err
	}
	return profile, nil
}

// BulkCreate inserts many profiles and returns successes/failures.
func (s *Service) BulkCreate(ctx context.Context, userID uuid.UUID, req BulkCreateRequest) ([]*Profile, []error) {
	profiles := make([]*Profile, 0, len(req.Profiles))
	errs := make([]error, len(req.Profiles))
	for i, payload := range req.Profiles {
		if err := s.validator.Struct(payload); err != nil {
			errs[i] = err
			continue
		}
		now := time.Now().UTC()
		p := &Profile{
			ID:        uuid.New(),
			UserID:    userID,
			FirstName: sanitizeField(s.sanitizer, payload.FirstName),
			LastName:  sanitizeField(s.sanitizer, payload.LastName),
			CreatedAt: now,
			UpdatedAt: now,
			Version:   1,
		}
		if err := assignOptionalFields(s.sanitizer, p, payload); err != nil {
			errs[i] = err
			continue
		}
		profiles = append(profiles, p)
	}
	if len(profiles) > 0 {
		_ = s.repo.BulkCreate(ctx, profiles)
	}
	return profiles, errs
}

// Get fetches a profile ensuring ownership.
func (s *Service) Get(ctx context.Context, id, userID uuid.UUID) (*Profile, error) {
	p, err := s.repo.GetByID(ctx, id, userID)
	if err != nil {
		return nil, err
	}
	if p == nil {
		return nil, ErrNotFound
	}
	return p, nil
}

// List returns paginated profiles.
func (s *Service) List(ctx context.Context, filter Filter) ([]Profile, int, error) {
	return s.repo.List(ctx, filter)
}

// Update performs PUT semantics.
func (s *Service) Update(ctx context.Context, id, userID uuid.UUID, req UpdateRequest) (*Profile, error) {
	if err := s.validator.Struct(req); err != nil {
		return nil, err
	}
	profile, err := s.repo.GetByID(ctx, id, userID)
	if err != nil {
		return nil, err
	}
	if profile == nil {
		return nil, ErrNotFound
	}
	if profile.Version != req.Version {
		return nil, ErrVersionConflict
	}
	profile.FirstName = sanitizeField(s.sanitizer, req.FirstName)
	profile.LastName = sanitizeField(s.sanitizer, req.LastName)
	if err := assignOptionalFields(s.sanitizer, profile, req.CreateRequest); err != nil {
		return nil, err
	}
	profile.Version++
	profile.UpdatedAt = time.Now().UTC()
	if err := s.repo.Update(ctx, profile); err != nil {
		return nil, err
	}
	return profile, nil
}

// Patch performs partial updates via a whitelist.
func (s *Service) Patch(ctx context.Context, id, userID uuid.UUID, req PatchRequest) (*Profile, error) {
	if err := s.validator.Struct(req); err != nil {
		return nil, err
	}
	allowed := map[string]bool{
		"first_name":    true,
		"last_name":     true,
		"bio":           true,
		"profile_image": true,
		"cover_image":   true,
		"date_of_birth": true,
		"phone":         true,
		"website":       true,
		"location":      true,
	}
	update := make(map[string]interface{})
	for k, v := range req.Fields {
		if !allowed[k] {
			continue
		}
		if str, ok := v.(string); ok {
			update[k] = sanitizeField(s.sanitizer, str)
		} else {
			update[k] = v
		}
	}
	if len(update) == 0 {
		return nil, errors.New("no valid fields to update")
	}
	updated, err := s.repo.Patch(ctx, id, userID, update, req.Version)
	if err != nil {
		return nil, err
	}
	return updated, nil
}

// Delete removes a profile (soft default).
func (s *Service) Delete(ctx context.Context, id, userID uuid.UUID, hard bool, version int) error {
	profile, err := s.repo.GetByID(ctx, id, userID)
	if err != nil {
		return err
	}
	if profile == nil {
		return ErrNotFound
	}
	if profile.Version != version {
		return ErrVersionConflict
	}
	return s.repo.Delete(ctx, id, userID, hard, version)
}

// BulkDelete removes multiple profiles at once.
func (s *Service) BulkDelete(ctx context.Context, userID uuid.UUID, req BulkDeleteRequest) (int, error) {
	if len(req.IDs) == 0 {
		return 0, errors.New("ids required")
	}
	return s.repo.BulkDelete(ctx, userID, req.IDs, req.Hard)
}

func assignOptionalFields(policy *bluemonday.Policy, profile *Profile, req CreateRequest) error {
	if req.Bio != nil {
		clean := sanitizeField(policy, *req.Bio)
		profile.Bio = &clean
	}
	if req.ProfileImage != nil {
		profile.ProfileImage = req.ProfileImage
	}
	if req.CoverImage != nil {
		profile.CoverImage = req.CoverImage
	}
	if req.Phone != nil {
		clean := sanitizeField(policy, *req.Phone)
		profile.Phone = &clean
	}
	if req.Website != nil {
		profile.Website = req.Website
	}
	if req.Location != nil {
		clean := sanitizeField(policy, *req.Location)
		profile.Location = &clean
	}
	if req.DateOfBirth != nil && *req.DateOfBirth != "" {
		t, err := time.Parse("2006-01-02", *req.DateOfBirth)
		if err != nil {
			return fmt.Errorf("invalid date_of_birth")
		}
		t = t.UTC()
		profile.DateOfBirth = &t
	}
	return nil
}

func sanitizeField(policy *bluemonday.Policy, val string) string {
	return strings.TrimSpace(policy.Sanitize(val))
}
