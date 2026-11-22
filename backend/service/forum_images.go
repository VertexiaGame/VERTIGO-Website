package service

import "vertexia-frontend/backend/models"

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
