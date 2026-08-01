package migrations

import (
	"context"

	dbinternal "github.com/xxnuo/open-coreui/backend/open_webui/internal"
	"github.com/xxnuo/open-coreui/backend/open_webui/migrations/versions"
)

type Definition struct {
	Revision     string
	DownRevision string
	Up           func(context.Context, *dbinternal.Handle) error
}

func Registered() []Definition {
	return []Definition{
		{
			Revision:     versions.V7E5B5DC7342BRevision,
			DownRevision: versions.V7E5B5DC7342BDownRevision,
			Up:           versions.UpgradeV7E5B5DC7342BInit,
		},
		{
			Revision:     versions.VC69F45358DB4Revision,
			DownRevision: versions.VC69F45358DB4DownRevision,
			Up:           versions.UpgradeVC69F45358DB4AddFolderTable,
		},
		{
			Revision:     versions.VC0FBF31CA0DBRevision,
			DownRevision: versions.VC0FBF31CA0DBDownRevision,
			Up:           versions.UpgradeVC0FBF31CA0DBUpdateFileTable,
		},
		{
			Revision:     versions.VC29FACFE716BRevision,
			DownRevision: versions.VC29FACFE716BDownRevision,
			Up:           versions.UpgradeVC29FACFE716BUpdateFileTablePath,
		},
		{
			Revision:     versions.V7826AB40B532Revision,
			DownRevision: versions.V7826AB40B532DownRevision,
			Up:           versions.UpgradeV7826AB40B532UpdateFileTable,
		},
		{
			Revision:     versions.V4ACE53FD72C8Revision,
			DownRevision: versions.V4ACE53FD72C8DownRevision,
			Up:           versions.UpgradeV4ACE53FD72C8UpdateFolderTableDatetime,
		},
		{
			Revision:     versions.VAF906E964978Revision,
			DownRevision: versions.VAF906E964978DownRevision,
			Up:           versions.UpgradeVAF906E964978AddFeedbackTable,
		},
		{
			Revision:     versions.VF1E2D3C4B5A6Revision,
			DownRevision: versions.VF1E2D3C4B5A6DownRevision,
			Up:           versions.UpgradeVF1E2D3C4B5A6AddAccessGrantTable,
		},
		{
			Revision:     versions.V922E7A387820Revision,
			DownRevision: versions.V922E7A387820DownRevision,
			Up:           versions.UpgradeV922E7A387820AddGroupTable,
		},
		{
			Revision:     versions.V37F288994C47Revision,
			DownRevision: versions.V37F288994C47DownRevision,
			Up:           versions.UpgradeV37F288994C47AddGroupMemberTable,
		},
		{
			Revision:     versions.V38D63C18F30FRevision,
			DownRevision: versions.V38D63C18F30FDownRevision,
			Up:           versions.UpgradeV38D63C18F30FAddOAuthSessionTable,
		},
		{
			Revision:     versions.V9F0C9CD09105Revision,
			DownRevision: versions.V9F0C9CD09105DownRevision,
			Up:           versions.UpgradeV9F0C9CD09105AddNoteTable,
		},
		{
			Revision:     versions.V374D2F66AF06Revision,
			DownRevision: versions.V374D2F66AF06DownRevision,
			Up:           versions.UpgradeV374D2F66AF06AddPromptHistoryTable,
		},
		{
			Revision:     versions.VA1B2C3D4E5F6Revision,
			DownRevision: versions.VA1B2C3D4E5F6DownRevision,
			Up:           versions.UpgradeVA1B2C3D4E5F6AddSkillTable,
		},
		{
			Revision:     versions.VD31026856C01Revision,
			DownRevision: versions.VD31026856C01DownRevision,
			Up:           versions.UpgradeVD31026856C01UpdateFolderTableData,
		},
		{
			Revision:     versions.CA81BD47C050Revision,
			DownRevision: versions.CA81BD47C050DownRevision,
			Up:           versions.UpgradeCA81BD47C050AddConfigTable,
		},
		{
			Revision:     versions.V3AF16A1C9FB6Revision,
			DownRevision: versions.V3AF16A1C9FB6DownRevision,
			Up:           versions.UpgradeV3AF16A1C9FB6UpdateUserTable,
		},
		{
			Revision:     versions.B10670C03DD5Revision,
			DownRevision: versions.B10670C03DD5DownRevision,
			Up:           versions.UpgradeB10670C03DD5UpdateUserTable,
		},
		{
			Revision:     versions.B2C3D4E5F6A7Revision,
			DownRevision: versions.B2C3D4E5F6A7DownRevision,
			Up:           versions.UpgradeB2C3D4E5F6A7AddScimColumnToUserTable,
		},
	}
}

func Run(ctx context.Context, db *dbinternal.Handle) error {
	for _, migration := range Registered() {
		if err := migration.Up(ctx, db); err != nil {
			return err
		}
	}
	return nil
}
