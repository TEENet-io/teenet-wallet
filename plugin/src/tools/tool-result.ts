// Shared helpers for formatting tool responses.

import type { ApprovalWatcher } from "../approval-watcher.js";

/** Standard MCP-style tool result. */
export interface ToolResult {
  content: Array<{ type: "text"; text: string }>;
}

/** Format any result as a tool response. */
export function jsonResult(data: unknown): ToolResult {
  return { content: [{ type: "text", text: JSON.stringify(data, null, 2) }] };
}

/**
 * Format a result that may contain pending_approval.
 * If pending, returns the approval info immediately (non-blocking)
 * and tracks the approval in the watcher for session-routed notifications.
 *
 * Backend uses two formats:
 * - Transfer: { status: "pending_approval", approval_id, approval_url }
 * - Policy/Contract/AddressBook: { pending: true, approval_id }
 * This helper handles both.
 */
export function approvalOrResult(
  result: { status?: string; pending?: boolean; approval_id?: number; approval_url?: string; [key: string]: unknown },
  getApprovalUrl: (id: number) => string,
  watcher?: ApprovalWatcher,
  sessionKey?: string,
  context?: string,
): ToolResult {
  const isPending = result.status === "pending_approval" || (result.pending === true && result.approval_id);
  if (isPending && result.approval_id) {
    if (watcher && sessionKey) {
      watcher.trackApproval(result.approval_id, sessionKey, context);
    }
    return jsonResult({
      status: "pending_approval",
      approval_id: result.approval_id,
      approval_url: result.approval_url || getApprovalUrl(result.approval_id),
      message: "Approval required. The user must approve via Passkey at the approval URL. You will be notified automatically when the approval is resolved.",
    });
  }
  return jsonResult(result);
}
