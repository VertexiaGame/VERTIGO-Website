package service

import "vertexia-frontend/backend/models"

func (s *ForumService) SavedImages(ownerID int, before int64, limit int) ([]models.SavedImage, error) {
	return s.forumRepo.SavedImages(ownerID, before, limit)
}

func (s *ForumService) SavedImage(ownerID int, id int64) (models.ForumImageUpload, error) {
	return s.forumRepo.SavedImage(ownerID, id)
}

func (s *ForumService) LoadApprovedImages(thread *models.ForumThread, replies []*models.ForumReply) error {
	return s.forumRepo.LoadApprovedImages(thread, replies)
}

func (s *ForumService) GetForumImage(id int, approved bool) (models.ForumImageUpload, error) {
	return s.forumRepo.GetForumImage(id, approved)
}

func (s *ForumService) PendingForumImages(after, limit int) ([]models.PendingForumImage, error) {
	return s.forumRepo.PendingForumImages(after, limit)
}

func (s *ForumService) ReviewForumImage(id int, approve bool) (bool, error) {
	return s.forumRepo.ReviewForumImage(id, approve)
}
