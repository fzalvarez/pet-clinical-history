package pets

import "context"

// OwnerOf expone el ownerUserID de una mascota.
// Se usa para evitar ciclos de imports entre módulos (pets <-> accessgrants).
func (s *Service) OwnerOf(ctx context.Context, petID string) (string, error) {
	p, err := s.GetByID(ctx, petID)
	if err != nil {
		return "", err
	}
	return p.OwnerUserID, nil
}
