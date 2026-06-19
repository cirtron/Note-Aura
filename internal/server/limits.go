package server

import (
	"io/fs"
	"path/filepath"
	"strconv"

	"note-aura/internal/db"
)

const bytesPerMB = 1 << 20

// userRole resolves a user's role, falling back to the built-in 'user' role.
func (s *Server) userRole(u *db.User) *db.Role {
	if u == nil {
		return nil
	}
	if r, err := s.db.GetRole(u.RoleSlug); err == nil {
		return r
	}
	r, _ := s.db.GetRole("user")
	return r
}

// canUseAIUser reports whether the user may use AI at all (own cloud key OR a
// role that permits the built-in Ollama).
func (s *Server) canUseAIUser(u *db.User) bool {
	if u == nil {
		return false
	}
	canAI, _ := s.db.UserAICapability(u.ID)
	return canAI
}

// overOllamaDaily reports whether the user has hit their daily Ollama-use limit.
// Only applies to users running on the built-in Ollama (cloud-key users and
// admins are never limited).
func (s *Server) overOllamaDaily(u *db.User) bool {
	if u == nil {
		return false
	}
	if _, usingOllama := s.db.UserAICapability(u.ID); !usingOllama {
		return false
	}
	limit := s.db.OllamaDailyLimit(u.ID)
	if limit <= 0 {
		return false
	}
	return s.db.OllamaUsedToday(u.ID) >= limit
}

// capacityLimitBytes returns a user's storage limit in bytes (0 = unlimited).
func (s *Server) capacityLimitBytes(u *db.User) int64 {
	if u == nil || u.IsAdmin {
		return 0
	}
	var limitMB int64
	if u.CapacityOverrideMB.Valid {
		limitMB = u.CapacityOverrideMB.Int64
	} else if r := s.userRole(u); r != nil {
		limitMB = r.CapacityMB
	}
	if limitMB <= 0 {
		return 0
	}
	return limitMB * bytesPerMB
}

// storageUsed is the user's current usage: note text + attachments + inline
// editor images on disk.
func (s *Server) storageUsed(u *db.User) int64 {
	used := s.db.StorageUsedBytes(u.ID)
	used += dirSize(filepath.Join(s.uploadsDir, "inline", strconv.FormatInt(u.ID, 10)))
	return used
}

// overCapacity reports whether adding addBytes would exceed the user's limit.
func (s *Server) overCapacity(u *db.User, addBytes int64) bool {
	limit := s.capacityLimitBytes(u)
	if limit <= 0 {
		return false
	}
	return s.storageUsed(u)+addBytes > limit
}

func dirSize(dir string) int64 {
	var total int64
	filepath.WalkDir(dir, func(_ string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		if info, e := d.Info(); e == nil {
			total += info.Size()
		}
		return nil
	})
	return total
}
