package user

import (
	"sort"
	"strings"
	"time"

	"github.com/arbovm/levenshtein"

	serr "github.com/tapglue/snaas/error"
	"github.com/tapglue/snaas/platform/flake"
)

type memService struct {
	users map[string]Map
}

// MemService returns a memory based Service implementation.
func MemService() Service {
	return &memService{
		users: map[string]Map{},
	}
}

func (s *memService) Count(ns string, opts QueryOptions) (int, error) {
	if err := s.Setup(ns); err != nil {
		return 0, err
	}

	return len(filterList(s.users[ns].ToList(), opts)), nil
}

func (s *memService) Put(ns string, input *User) (*User, error) {
	if err := s.Setup(ns); err != nil {
		return nil, err
	}

	if err := input.Validate(); err != nil {
		return nil, err
	}

	var (
		bucket = s.users[ns]
		now    = time.Now().UTC()
	)

	if input.ID == 0 {
		id, err := flake.NextID(flakeNamespace(ns))
		if err != nil {
			return nil, err
		}

		if input.CreatedAt.IsZero() {
			input.CreatedAt = now
		}
		input.ID = id
	} else {
		keep := false

		for _, u := range bucket {
			if u.ID == input.ID {
				keep = true
				input.CreatedAt = u.CreatedAt
			}
		}

		if !keep {
			return nil, ErrNotFound
		}
	}

	input.UpdatedAt = now
	bucket[input.ID] = copy(input)

	return copy(input), nil
}

func (s *memService) PutLastRead(ns string, userID uint64, ts time.Time) error {
	if err := s.Setup(ns); err != nil {
		return err
	}

	u, ok := s.users[ns][userID]
	if ok {
		u.LastRead = ts.UTC()
		s.users[ns][userID] = u
	}

	return nil
}
func (s *memService) Query(ns string, opts QueryOptions) (List, error) {
	if err := s.Setup(ns); err != nil {
		return nil, err
	}

	us := filterList(s.users[ns].ToList(), opts)

	if opts.Limit > 0 && len(us) > opts.Limit {
		us = us[:opts.Limit]
	}

	return us, nil
}

func (s *memService) Search(ns string, opts QueryOptions) (List, error) {
	if err := s.Setup(ns); err != nil {
		return nil, err
	}

	if opts.Query == "" {
		return nil, serr.Wrap(serr.ErrInvalidQuery, "param is empty")
	}

	us := s.users[ns].ToList()

	sort.SliceStable(us, func(i, j int) bool {
		return levenshtein.Distance(opts.Query, us[i].Username) < levenshtein.Distance(opts.Query, us[j].Username)
	})

	fs := List{}

	for _, u := range us {
		if levenshtein.Distance(opts.Query, u.Username) < 8 {
			fs = append(fs, u)
		}
	}

	if int(opts.Offset) > len(us) {
		return List{}, nil
	}

	if opts.Limit == 0 && opts.Offset == 0 {
		return us, nil
	}

	return us[int(opts.Offset) : int(opts.Offset)+opts.Limit], nil
}

func (s *memService) Setup(ns string) error {
	if _, ok := s.users[ns]; !ok {
		s.users[ns] = Map{}
	}

	return nil
}

func (s *memService) Teardown(ns string) error {
	if _, ok := s.users[ns]; ok {
		delete(s.users, ns)
	}

	return nil
}

func contains(s string, ts ...string) bool {
	if len(ts) == 0 {
		return true
	}

	keep := false

	for _, t := range ts {
		if keep = strings.Contains(s, t); keep {
			break
		}
	}

	return keep
}

func copy(u *User) *User {
	old := *u
	return &old
}

func filterList(us List, opts QueryOptions) List {
	rs := List{}

	for _, u := range us {
		if !inTypes(u.CustomID, opts.CustomIDs) {
			continue
		}

		if opts.Deleted != nil && u.Deleted != *opts.Deleted {
			continue
		}

		if !inTypes(u.Email, opts.Emails) {
			continue
		}

		if opts.Enabled != nil && u.Enabled != *opts.Enabled {
			continue
		}

		if !inIDs(u.ID, opts.IDs) {
			continue
		}

		if opts.SocialIDs != nil {
			keep := false

			for platform, ids := range opts.SocialIDs {
				if _, ok := u.SocialIDs[platform]; !ok {
					continue
				}

				if !inTypes(u.SocialIDs[platform], ids) {
					continue
				}

				keep = true
			}

			if !keep {
				continue
			}
		}

		if !inTypes(u.Username, opts.Usernames) {
			continue
		}

		rs = append(rs, u)
	}

	return rs
}

func inIDs(id uint64, ids []uint64) bool {
	if len(ids) == 0 {
		return true
	}

	keep := false

	for _, i := range ids {
		if id == i {
			keep = true
			break
		}
	}

	return keep
}

func inTypes(ty string, ts []string) bool {
	if len(ts) == 0 {
		return true
	}

	keep := false

	for _, t := range ts {
		if ty == t {
			keep = true
			break
		}
	}

	return keep
}
