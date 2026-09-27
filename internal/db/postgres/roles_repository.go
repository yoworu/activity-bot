package postgres

import (
	db "activity-bot/internal/db/postgres/sqlc"
	"activity-bot/internal/roles"
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct {
	pool    *pgxpool.Pool
	queries *db.Queries
}

func NewRolesRepository(
	pool *pgxpool.Pool,
	queries *db.Queries,
) *Repository {
	return &Repository{
		pool:    pool,
		queries: queries,
	}
}

func (r *Repository) CreateRoleReservation(
	ctx context.Context,
	chatID,
	userID,
	roleID int64,
) error {
	return r.queries.CreateRoleReservation(ctx, db.CreateRoleReservationParams{
		ChatID: chatID,
		UserID: userID,
		RoleID: roleID,
	})
}

func (r *Repository) DeleteRoleReservation(
	ctx context.Context,
	chatID int64,
	roleID int64,
) error {
	return r.queries.DeleteRoleReservation(ctx, db.DeleteRoleReservationParams{
		ChatID: chatID,
		RoleID: roleID,
	})
}

func (r *Repository) GetRoleReservation(
	ctx context.Context,
	chatID int64,
	roleID int64,
) (roles.RoleReservation, error) {
	rr, err := r.queries.GetRoleReservation(ctx, db.GetRoleReservationParams{
		ChatID: chatID,
		RoleID: roleID,
	})
	if err != nil {
		return roles.RoleReservation{}, err
	}
	return mapRoleReservation(rr), nil

}

func (r *Repository) ListRoleReservations(
	ctx context.Context,
	chatID int64,
) ([]roles.RoleReservation, error) {
	reservations, err := r.queries.ListRoleReservations(ctx, chatID)
	if err != nil {
		return nil, err
	}
	return mapList(reservations, func(row db.ListRoleReservationsRow) roles.RoleReservation {
		rr := mapRoleReservation(row.RoleReservation)
		rr.Role = mapRole(row.Role)
		return rr
	}), err
}

func (r *Repository) CreateRoleTemplate(
	ctx context.Context,
	chatID int64,
	fandomName string,
	categories []roles.Category,
) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	q := r.queries.WithTx(tx)

	if err := q.DeleteFandom(ctx, db.DeleteFandomParams{
		ChatID: chatID,
		Name:   fandomName,
	}); err != nil {
		return err
	}

	fandom, err := q.GetOrCreateFandom(ctx, db.GetOrCreateFandomParams{
		ChatID: chatID,
		Name:   fandomName,
	})
	if err != nil {
		return err
	}

	for _, category := range categories {
		categoryRow, err := q.CreateRoleCategory(
			ctx,
			db.CreateRoleCategoryParams{
				FandomID: fandom.ID,
				Name:     category.Name,
			},
		)
		if err != nil {
			return err
		}

		for _, role := range category.Roles {
			roleRow, err := q.CreateRole(
				ctx,
				db.CreateRoleParams{
					CategoryID: categoryRow.ID,
					Name:       role.Name,
					Emoji:      text(role.Emoji),
				},
			)
			if err != nil {
				return err
			}

			for _, alias := range role.Aliases {
				_, err = q.CreateRoleAlias(
					ctx,
					db.CreateRoleAliasParams{
						RoleID: roleRow.ID,
						Name:   alias,
					},
				)
				if err != nil {
					return err
				}
			}
		}
	}

	return tx.Commit(ctx)
}

func (r *Repository) GetRoleByName(
	ctx context.Context,
	chatID int64,
	fandomName string,
	categoryName string,
	roleName string,
) (roles.Role, error) {
	row, err := r.queries.GetRoleByName(ctx, db.GetRoleByNameParams{
		ChatID: chatID,
		Name:   fandomName,
		Name_2: categoryName,
		Name_3: roleName,
	})
	if err != nil {
		return roles.Role{}, err
	}

	return mapRole(row), nil
}

func (r *Repository) GetRoleByAlias(
	ctx context.Context,
	chatID int64,
	fandomName string,
	alias string,
) (roles.Role, error) {
	row, err := r.queries.GetRoleByAlias(ctx, db.GetRoleByAliasParams{
		ChatID: chatID,
		Name:   fandomName,
		Name_2: alias,
	})
	if err != nil {
		return roles.Role{}, err
	}

	return mapRole(row), nil
}

func (r *Repository) GetRoleByNameOrAlias(
	ctx context.Context,
	chatID int64,
	fandomName string,
	name string,
) (roles.Role, error) {
	row, err := r.queries.GetRoleByNameOrAlias(ctx, db.GetRoleByNameOrAliasParams{
		ChatID:  chatID,
		Lower:   fandomName,
		Lower_2: name,
	})
	if err != nil {
		return roles.Role{}, err
	}

	return mapRole(row), nil
}

func (r *Repository) CreateRole(
	ctx context.Context,
	categoryID int64,
	name string,
	emoji string,
) (roles.Role, error) {
	row, err := r.queries.CreateRole(ctx, db.CreateRoleParams{
		CategoryID: categoryID,
		Name:       name,
		Emoji:      text(emoji),
	})
	if err != nil {
		return roles.Role{}, err
	}

	return mapRole(row), nil
}

func (r *Repository) CreateRoleAlias(
	ctx context.Context,
	roleID int64,
	name string,
) error {
	_, err := r.queries.CreateRoleAlias(ctx, db.CreateRoleAliasParams{
		RoleID: roleID,
		Name:   name,
	})

	return err
}

func (r *Repository) GetFandom(
	ctx context.Context,
	chatID int64,
	name string,
) (roles.Fandom, error) {
	row, err := r.queries.GetFandom(ctx, db.GetFandomParams{
		ChatID: chatID,
		Name:   name,
	})
	if err != nil {
		return roles.Fandom{}, err
	}

	return mapFandom(row), nil
}

func (r *Repository) GetOrCreateFandom(
	ctx context.Context,
	chatID int64,
	name string,
) (roles.Fandom, error) {
	row, err := r.queries.GetOrCreateFandom(ctx, db.GetOrCreateFandomParams{
		ChatID: chatID,
		Name:   name,
	})
	if err != nil {
		return roles.Fandom{}, err
	}

	return mapFandom(row), nil
}

func (r *Repository) GetRoleCategory(
	ctx context.Context,
	fandomID int64,
	name string,
) (roles.Category, error) {
	row, err := r.queries.GetRoleCategory(ctx, db.GetRoleCategoryParams{
		FandomID: fandomID,
		Name:     name,
	})
	if err != nil {
		return roles.Category{}, err
	}

	return mapCategory(row), nil
}

func (r *Repository) ListRoleCategories(
	ctx context.Context,
	fandomID int64,
) ([]roles.Category, error) {
	rows, err := r.queries.ListRoleCategories(ctx, fandomID)
	if err != nil {
		return nil, err
	}

	result := make([]roles.Category, 0, len(rows))

	for _, row := range rows {
		result = append(result, mapCategory(row))
	}

	return result, nil
}

func (r *Repository) CreateRoleCategory(
	ctx context.Context,
	fandomID int64,
	name string,
) (roles.Category, error) {
	row, err := r.queries.CreateRoleCategory(ctx, db.CreateRoleCategoryParams{
		FandomID: fandomID,
		Name:     name,
	})
	if err != nil {
		return roles.Category{}, err
	}

	return mapCategory(row), nil
}

func (r *Repository) GetRoleTemplate(
	ctx context.Context,
	chatID int64,
	fandomName string,
) (roles.Fandom, error) {
	rows, err := r.queries.GetFandomWithRoles(
		ctx,
		db.GetFandomWithRolesParams{
			ChatID: chatID,
			Name:   fandomName,
		},
	)
	if err != nil {
		return roles.Fandom{}, err
	}

	if len(rows) == 0 {
		return roles.Fandom{}, pgx.ErrNoRows
	}

	fandom := roles.Fandom{
		ID:   rows[0].FandomID,
		Name: rows[0].FandomName,
	}

	categoryIndexes := make(map[int64]int)
	roleIndexes := make(map[int64]struct {
		categoryIndex int
		roleIndex     int
	})

	for _, row := range rows {
		categoryIndex, exists := categoryIndexes[row.CategoryID.Int64]
		if !exists {
			categoryIndex = len(fandom.Categories)

			fandom.Categories = append(
				fandom.Categories,
				roles.Category{
					ID:    row.CategoryID.Int64,
					Name:  row.CategoryName.String,
					Emoji: row.CategoryEmoji.String,
				},
			)

			categoryIndexes[row.CategoryID.Int64] = categoryIndex
		}

		roleKey := row.RoleID

		roleIndexData, exists := roleIndexes[roleKey.Int64]
		if !exists {
			roleIndex := len(fandom.Categories[categoryIndex].Roles)

			fandom.Categories[categoryIndex].Roles = append(
				fandom.Categories[categoryIndex].Roles,
				roles.Role{
					ID:      row.RoleID.Int64,
					Name:    row.RoleName.String,
					Emoji:   row.RoleEmoji.String,
					Aliases: []string{},
				},
			)

			roleIndexes[roleKey.Int64] = struct {
				categoryIndex int
				roleIndex     int
			}{
				categoryIndex: categoryIndex,
				roleIndex:     roleIndex,
			}

			roleIndexData = roleIndexes[roleKey.Int64]
		}

		if row.AliasID.Valid {
			role := &fandom.Categories[roleIndexData.categoryIndex].Roles[roleIndexData.roleIndex]

			role.Aliases = append(
				role.Aliases,
				row.AliasName.String,
			)
		}
	}

	return fandom, nil
}

func (r *Repository) ListRoleTemplates(
	ctx context.Context,
	chatID int64,
) ([]roles.Fandom, error) {
	rows, err := r.queries.ListRoleTemplates(ctx, chatID)
	if err != nil {
		return nil, err
	}

	result := make([]roles.Fandom, 0)

	fandomIndexes := make(map[int64]int)
	categoryIndexes := make(map[int64]int)
	roleIndexes := make(map[int64]struct {
		fandomIndex   int
		categoryIndex int
		roleIndex     int
	})

	for _, row := range rows {
		// Fandom
		fandomIndex, exists := fandomIndexes[row.FandomID]
		if !exists {
			fandomIndex = len(result)

			result = append(result, roles.Fandom{
				ID:         row.FandomID,
				Name:       row.FandomName,
				Categories: []roles.Category{},
			})

			fandomIndexes[row.FandomID] = fandomIndex
		}

		// Category
		categoryKey := row.CategoryID

		categoryIndex, exists := categoryIndexes[categoryKey]
		if !exists {
			categoryIndex = len(result[fandomIndex].Categories)

			result[fandomIndex].Categories = append(
				result[fandomIndex].Categories,
				roles.Category{
					ID:    row.CategoryID,
					Name:  row.CategoryName,
					Roles: []roles.Role{},
				},
			)

			categoryIndexes[categoryKey] = categoryIndex
		}

		// Role
		roleKey := row.RoleID

		roleIndexData, exists := roleIndexes[roleKey]
		if !exists {
			roleIndex := len(
				result[fandomIndex].Categories[categoryIndex].Roles,
			)

			result[fandomIndex].Categories[categoryIndex].Roles = append(
				result[fandomIndex].Categories[categoryIndex].Roles,
				roles.Role{
					ID:      row.RoleID,
					Name:    row.RoleName,
					Emoji:   row.RoleEmoji.String,
					Aliases: []string{},
				},
			)

			roleIndexes[roleKey] = struct {
				fandomIndex   int
				categoryIndex int
				roleIndex     int
			}{
				fandomIndex:   fandomIndex,
				categoryIndex: categoryIndex,
				roleIndex:     roleIndex,
			}

			roleIndexData = roleIndexes[roleKey]
		}

		// Alias
		if row.AliasID.Valid {
			role := &result[roleIndexData.fandomIndex].Categories[roleIndexData.categoryIndex].Roles[roleIndexData.roleIndex]

			role.Aliases = append(
				role.Aliases,
				row.AliasName.String,
			)
		}
	}

	return result, nil
}
