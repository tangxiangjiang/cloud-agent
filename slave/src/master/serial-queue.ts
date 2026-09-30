// Copyright (c) 2026 smarttang
// SPDX-License-Identifier: MIT

/** Per-key serial promise queue (start/stop/restart for one slave). */
export class SerialQueue {
  private readonly tails = new Map<string, Promise<unknown>>();

  enqueue<T>(key: string, fn: () => Promise<T>): Promise<T> {
    const prev = this.tails.get(key) ?? Promise.resolve();
    const next = prev.then(
      () => fn(),
      () => fn(),
    );
    this.tails.set(
      key,
      next.then(
        () => undefined,
        () => undefined,
      ),
    );
    return next;
  }
}
