// Copyright (c) 2026 smarttang
// SPDX-License-Identifier: MIT

/** Stable control-plane error codes (doc/slave-master.md §14). */
export type MasterErrorCode =
  | "slave_disabled"
  | "slave_limit"
  | "start_busy"
  | "cwd_denied"
  | "repo_conflict"
  | "not_stopped"
  | "id_immutable"
  | "unknown_slave"
  | "busy"
  | "cancel_skipped_gateway_unreachable";

export class MasterControlError extends Error {
  readonly code: MasterErrorCode;

  constructor(code: MasterErrorCode, message: string) {
    super(message);
    this.name = "MasterControlError";
    this.code = code;
  }
}
