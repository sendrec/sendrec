import { formatDuration } from "../../utils/format";

interface Comment {
  id: string;
  authorName: string;
  body: string;
  isPrivate: boolean;
  isOwner: boolean;
  createdAt: string;
  videoTimestamp: number | null;
}

function getInitials(name: string): string {
  if (!name) return "?";
  return name
    .split(" ")
    .map((w) => w[0])
    .join("")
    .toUpperCase()
    .slice(0, 2);
}

function relativeTime(isoDate: string): string {
  const diff = Date.now() - new Date(isoDate).getTime();
  const minutes = Math.floor(diff / 60000);
  if (minutes < 1) return "just now";
  if (minutes < 60) return `${minutes}m ago`;
  const hours = Math.floor(minutes / 60);
  if (hours < 24) return `${hours}h ago`;
  const days = Math.floor(hours / 24);
  if (days < 30) return `${days}d ago`;
  return new Date(isoDate).toLocaleDateString("en-GB");
}

interface CommentsSectionProps {
  comments: Comment[];
  isViewer: boolean;
  onDeleteComment: (commentId: string) => void;
}

export function CommentsSection({
  comments,
  isViewer,
  onDeleteComment,
}: CommentsSectionProps) {
  return (
    <div className="video-detail-section">
      <h2 className="video-detail-section-title">
        Comments ({comments.length})
      </h2>
      {comments.length === 0 ? (
        <p style={{ color: "var(--color-text-secondary)", fontSize: 13 }}>
          No comments yet.
        </p>
      ) : (
        comments.map((comment) => (
          <div key={comment.id} className="comment-item">
            <div className="comment-avatar">
              {getInitials(comment.authorName)}
            </div>
            <div className="comment-content">
              <div className="comment-header">
                <span className="comment-author">
                  {comment.authorName || "Anonymous"}
                </span>
                <span className="comment-date">
                  {relativeTime(comment.createdAt)}
                </span>
                {comment.videoTimestamp !== null && (
                  <span className="comment-timestamp">
                    @{formatDuration(comment.videoTimestamp)}
                  </span>
                )}
                {comment.isPrivate && (
                  <span className="comment-private">Private</span>
                )}
              </div>
              <div className="comment-body">{comment.body}</div>
            </div>
            {!isViewer && (
              <button
                className="comment-delete"
                onClick={() => onDeleteComment(comment.id)}
                aria-label="Delete comment"
              >
                Delete
              </button>
            )}
          </div>
        ))
      )}
    </div>
  );
}
