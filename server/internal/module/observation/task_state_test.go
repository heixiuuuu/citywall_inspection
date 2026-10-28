package observation

import (
	"context"
	"errors"
	"testing"

	"citywall/server/internal/module/task"
)

func TestEndedTaskRejectsCollectionChanges(t *testing.T) {
	for _, status := range []string{task.StatusCancelled, task.StatusCompleted} {
		for _, action := range []string{"update", "conclusion", "submit", "remove", "upload"} {
			t.Run(status+"/"+action, func(t *testing.T) {
				s, repo := newObsService()
				ctx := context.Background()
				pointID := int64(1)
				o, err := s.Create(ctx, inspector(), SourceTask, &pointID, "原位置")
				if err != nil {
					t.Fatal(err)
				}
				image, err := s.AddImage(ctx, inspector(), o.ID, "image/png", pngBytes(), ImageRoleOverview, "")
				if err != nil {
					t.Fatal(err)
				}
				if _, err := s.SaveConclusion(ctx, inspector(), o.ID, 1, ConclusionInput{Outcome: OutcomeNormal}); err != nil {
					t.Fatal(err)
				}
				repo.taskPoints[pointID].TaskStatus = status
				switch action {
				case "update":
					_, err = s.Update(ctx, inspector(), o.ID, 2, PatchInput{LocationText: "不得保存"})
				case "conclusion":
					_, err = s.SaveConclusion(ctx, inspector(), o.ID, 2, ConclusionInput{Outcome: OutcomeUndetermined})
				case "submit":
					_, err = s.Submit(ctx, inspector(), o.ID, 2)
				case "remove":
					err = s.RemoveImage(ctx, inspector(), o.ID, image.ID)
				case "upload":
					_, err = s.AddImage(ctx, inspector(), o.ID, "image/png", pngBytes(), ImageRoleDetail, "")
				}
				if !errors.Is(err, ErrInvalidState) {
					t.Fatalf("err=%v", err)
				}
				if o.Revision != 2 || o.Status != ObsDraft || o.LocationText != "原位置" || len(repo.subs) != 0 || len(repo.concl) != 1 || len(repo.links[o.ID]) != 1 {
					t.Fatal("ended task collection changed persisted evidence")
				}
			})
		}
	}
}
