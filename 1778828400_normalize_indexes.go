package pgbase

import (
	"slices"

	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/core"
)

func init() {
	var items = core.SystemMigrations.Items()
	i := slices.IndexFunc(items, func(item *core.Migration) bool {
		return item.File == "1778828400_normalize_indexes.go"
	})
	item := items[i]
	up := item.Up
	item.Up = func(txApp core.App) error {
		tx := txApp.DB().(*dbx.Tx)
		_, ok := tx.Builder.(*Builder)
		if !ok {
			return up(txApp)
		}

		collections, err := txApp.FindAllCollections()
		if err != nil {
			return err
		}

		for _, collection := range collections {
			// existing system collection indexes can't be modified and view don't have indexes
			if collection.System || collection.IsView() {
				continue
			}

			// resave to trigger indexes normalization
			err = txApp.Save(collection)
			if err != nil {
				return err
			}
		}

		return nil
	}
}
