package folder

type CreateFolderRequest struct {
	Name     string `json:"name" binding:"required,min=1,max=100"`
	ParentID string `json:"parentId" binding:"omitempty,uuid"`
}

type RenameFolderRequest struct {
	Name string `json:"name" binding:"required,min=1,max=100"`
}

type MoveFolderRequest struct {
	ParentID string `json:"parentId" binding:"omitempty,uuid"`
}
