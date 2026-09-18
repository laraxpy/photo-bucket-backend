package service

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/laraxpy/photo-bucket-backend/internal/apperror"
	"github.com/laraxpy/photo-bucket-backend/internal/model/file"
)

func newTestFolderService() (FolderService, *fakeFolderStore, *fakeFileStore) {
	folders := newFakeFolderStore()
	files := newFakeFileStore()
	return NewFolderService(folders, files), folders, files
}

func appErrorCode(t *testing.T, err error) apperror.ErrorCode {
	t.Helper()
	appErr := apperror.From(err)
	if appErr == nil {
		t.Fatal("expected an error, got nil")
	}
	return appErr.Code
}

func TestFolderService_Create(t *testing.T) {
	ctx := context.Background()
	userID := uuid.New()

	t.Run("creates a root folder", func(t *testing.T) {
		svc, _, _ := newTestFolderService()
		f, err := svc.Create(ctx, userID, "Vacaciones", "")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if f.ParentID != nil {
			t.Errorf("expected root folder to have nil ParentID, got %v", f.ParentID)
		}
	})

	t.Run("rejects duplicate name in the same parent", func(t *testing.T) {
		svc, _, _ := newTestFolderService()
		if _, err := svc.Create(ctx, userID, "Vacaciones", ""); err != nil {
			t.Fatalf("unexpected error on first create: %v", err)
		}
		_, err := svc.Create(ctx, userID, "Vacaciones", "")
		if err == nil {
			t.Fatal("expected a conflict error for duplicate name")
		}
		if code := appErrorCode(t, err); code != apperror.CodeConflict {
			t.Errorf("Code = %v, want %v", code, apperror.CodeConflict)
		}
	})

	t.Run("rejects a parentId that belongs to another user", func(t *testing.T) {
		svc, folders, _ := newTestFolderService()
		otherUser := uuid.New()
		other, _ := svc.Create(ctx, otherUser, "Privado", "")
		_ = folders // parent already stored via svc.Create

		_, err := svc.Create(ctx, userID, "Intento", other.ID.String())
		if err == nil {
			t.Fatal("expected an error when parent belongs to another user")
		}
		if code := appErrorCode(t, err); code != apperror.CodeNotFound {
			t.Errorf("Code = %v, want %v (should not leak existence of another user's folder)", code, apperror.CodeNotFound)
		}
	})

	t.Run("rejects an invalid parentId format", func(t *testing.T) {
		svc, _, _ := newTestFolderService()
		_, err := svc.Create(ctx, userID, "X", "not-a-uuid")
		if code := appErrorCode(t, err); code != apperror.CodeBadRequest {
			t.Errorf("Code = %v, want %v", code, apperror.CodeBadRequest)
		}
	})
}

func TestFolderService_Move(t *testing.T) {
	ctx := context.Background()
	userID := uuid.New()

	t.Run("moves a folder to a new parent", func(t *testing.T) {
		svc, _, _ := newTestFolderService()
		parentA, _ := svc.Create(ctx, userID, "A", "")
		parentB, _ := svc.Create(ctx, userID, "B", "")
		child, _ := svc.Create(ctx, userID, "Child", parentA.ID.String())

		moved, err := svc.Move(ctx, userID, child.ID, parentB.ID.String())
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if moved.ParentID == nil || *moved.ParentID != parentB.ID {
			t.Errorf("expected folder to be moved under B, got parentId=%v", moved.ParentID)
		}
	})

	t.Run("rejects moving a folder into itself", func(t *testing.T) {
		svc, _, _ := newTestFolderService()
		f, _ := svc.Create(ctx, userID, "Self", "")

		_, err := svc.Move(ctx, userID, f.ID, f.ID.String())
		if code := appErrorCode(t, err); code != apperror.CodeBadRequest {
			t.Errorf("Code = %v, want %v", code, apperror.CodeBadRequest)
		}
	})

	t.Run("rejects moving a folder into its own descendant (cycle)", func(t *testing.T) {
		svc, _, _ := newTestFolderService()
		grandparent, _ := svc.Create(ctx, userID, "Grandparent", "")
		parent, _ := svc.Create(ctx, userID, "Parent", grandparent.ID.String())
		child, _ := svc.Create(ctx, userID, "Child", parent.ID.String())

		// Trying to move "Grandparent" under its own grandchild must fail.
		_, err := svc.Move(ctx, userID, grandparent.ID, child.ID.String())
		if err == nil {
			t.Fatal("expected an error preventing a cycle in the folder tree")
		}
		if code := appErrorCode(t, err); code != apperror.CodeBadRequest {
			t.Errorf("Code = %v, want %v", code, apperror.CodeBadRequest)
		}
	})

	t.Run("moving to the current parent is a no-op", func(t *testing.T) {
		svc, _, _ := newTestFolderService()
		parent, _ := svc.Create(ctx, userID, "Parent", "")
		child, _ := svc.Create(ctx, userID, "Child", parent.ID.String())

		got, err := svc.Move(ctx, userID, child.ID, parent.ID.String())
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got.ParentID == nil || *got.ParentID != parent.ID {
			t.Errorf("expected folder to remain under the same parent")
		}
	})

	t.Run("rejects a name collision at the destination", func(t *testing.T) {
		svc, _, _ := newTestFolderService()
		parentA, _ := svc.Create(ctx, userID, "A", "")
		parentB, _ := svc.Create(ctx, userID, "B", "")
		_, _ = svc.Create(ctx, userID, "Fotos", parentB.ID.String())
		child, _ := svc.Create(ctx, userID, "Fotos", parentA.ID.String())

		_, err := svc.Move(ctx, userID, child.ID, parentB.ID.String())
		if code := appErrorCode(t, err); code != apperror.CodeConflict {
			t.Errorf("Code = %v, want %v", code, apperror.CodeConflict)
		}
	})
}

func TestFolderService_Delete(t *testing.T) {
	ctx := context.Background()
	userID := uuid.New()

	t.Run("deletes an empty folder", func(t *testing.T) {
		svc, _, _ := newTestFolderService()
		f, _ := svc.Create(ctx, userID, "Empty", "")

		if err := svc.Delete(ctx, userID, f.ID); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if _, err := svc.GetByID(ctx, userID, f.ID); err == nil {
			t.Error("expected folder to be gone after delete")
		}
	})

	t.Run("refuses to delete a folder with subfolders", func(t *testing.T) {
		svc, _, _ := newTestFolderService()
		parent, _ := svc.Create(ctx, userID, "Parent", "")
		_, _ = svc.Create(ctx, userID, "Child", parent.ID.String())

		err := svc.Delete(ctx, userID, parent.ID)
		if code := appErrorCode(t, err); code != apperror.CodeConflict {
			t.Errorf("Code = %v, want %v", code, apperror.CodeConflict)
		}
	})

	t.Run("refuses to delete a folder that still has files", func(t *testing.T) {
		svc, _, files := newTestFolderService()
		f, _ := svc.Create(ctx, userID, "HasFiles", "")

		// Simulate a file living inside this folder.
		fileID := uuid.New()
		folderID := f.ID
		files.files[fileID] = file.File{ID: fileID, UserID: userID, FolderID: &folderID}

		err := svc.Delete(ctx, userID, f.ID)
		if code := appErrorCode(t, err); code != apperror.CodeConflict {
			t.Errorf("Code = %v, want %v", code, apperror.CodeConflict)
		}
	})
}

func TestFolderService_OwnershipIsEnforced(t *testing.T) {
	ctx := context.Background()
	owner := uuid.New()
	intruder := uuid.New()

	svc, _, _ := newTestFolderService()
	f, _ := svc.Create(ctx, owner, "Secreto", "")

	t.Run("GetByID hides other users' folders as 404", func(t *testing.T) {
		_, err := svc.GetByID(ctx, intruder, f.ID)
		if code := appErrorCode(t, err); code != apperror.CodeNotFound {
			t.Errorf("Code = %v, want %v", code, apperror.CodeNotFound)
		}
	})

	t.Run("Rename is rejected for a non-owner", func(t *testing.T) {
		_, err := svc.Rename(ctx, intruder, f.ID, "Hackeado")
		if code := appErrorCode(t, err); code != apperror.CodeNotFound {
			t.Errorf("Code = %v, want %v", code, apperror.CodeNotFound)
		}
	})

	t.Run("Delete is rejected for a non-owner", func(t *testing.T) {
		err := svc.Delete(ctx, intruder, f.ID)
		if code := appErrorCode(t, err); code != apperror.CodeNotFound {
			t.Errorf("Code = %v, want %v", code, apperror.CodeNotFound)
		}
	})
}
