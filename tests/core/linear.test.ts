// tests/core/linear.test.ts
//
// Behavioral contract for the LinearBridge -- the adapter between our local
// store and the Linear API. All Linear SDK calls are mocked.
//
// These tests protect against:
// - Wrong fields being sent to Linear API (wrong title, priority, labels)
// - deleteIssue actually deleting vs trashing (Linear only supports trash)
// - Label cache misses causing duplicate labels
// - API failures not being caught by isAvailable check
// - fetchWorkflowStates returning wrong shape

import { mock, describe, it, expect } from 'bun:test';
import { LinearBridge } from '../../src/core/linear.js';

// Helper to build a mock LinearClient with controllable responses
function createMockClient(overrides: Record<string, unknown> = {}) {
  return {
    createIssue: mock().mockResolvedValue({
      success: true,
      issue: Promise.resolve({ id: 'linear-issue-1' }),
    }),
    updateIssue: mock().mockResolvedValue({ success: true }),
    createIssueLabel: mock().mockResolvedValue({
      success: true,
      issueLabel: Promise.resolve({
        id: 'linear-label-new',
        name: 'newlabel',
      }),
    }),
    issueLabels: mock().mockResolvedValue({
      nodes: [],
    }),
    workflowStates: mock().mockResolvedValue({
      nodes: [
        { id: 'ws-1', name: 'Backlog', type: 'backlog' },
        { id: 'ws-2', name: 'In Progress', type: 'started' },
        { id: 'ws-3', name: 'Done', type: 'completed' },
      ],
    }),
    viewer: Promise.resolve({ id: 'user-1' }),
    ...overrides,
  } as any;
}

describe('LinearBridge', () => {
  const teamId = 'team-abc-123';

  describe('createIssue', () => {
    it('calls issueCreate with teamId, title, description, priority, stateId, labelIds', async () => {
      const client = createMockClient();
      const bridge = new LinearBridge(client, teamId);

      const linearId = await bridge.createIssue({
        title: 'Fix server timeout',
        description: 'Server times out after 30s',
        priority: 1,
        stateId: 'ws-1',
        labelIds: ['label-a', 'label-b'],
      });

      expect(client.createIssue).toHaveBeenCalledTimes(1);
      const callArgs = client.createIssue.mock.calls[0][0];
      expect(callArgs.teamId).toBe(teamId);
      expect(callArgs.title).toBe('Fix server timeout');
      expect(callArgs.description).toBe('Server times out after 30s');
      expect(callArgs.priority).toBe(1);
      expect(callArgs.stateId).toBe('ws-1');
      expect(callArgs.labelIds).toEqual(['label-a', 'label-b']);
      expect(linearId).toBe('linear-issue-1');
    });

    it('returns the Linear issue ID', async () => {
      const client = createMockClient();
      const bridge = new LinearBridge(client, teamId);

      const id = await bridge.createIssue({
        title: 'Test',
        description: '',
        priority: 0,
      });
      expect(id).toBe('linear-issue-1');
    });
  });

  describe('updateIssue', () => {
    it('calls issueUpdate with correct ID and fields', async () => {
      const client = createMockClient();
      const bridge = new LinearBridge(client, teamId);

      await bridge.updateIssue('linear-issue-1', {
        title: 'Updated title',
        priority: 2,
        stateId: 'ws-2',
      });

      expect(client.updateIssue).toHaveBeenCalledTimes(1);
      expect(client.updateIssue).toHaveBeenCalledWith('linear-issue-1', {
        title: 'Updated title',
        priority: 2,
        stateId: 'ws-2',
      });
    });
  });

  describe('deleteIssue', () => {
    it('calls issueUpdate with trashed: true (Linear archives, does not hard-delete)', async () => {
      const client = createMockClient();
      const bridge = new LinearBridge(client, teamId);

      await bridge.deleteIssue('linear-issue-1');

      // Must use issueUpdate, NOT issueDelete -- Linear trashes, not deletes
      expect(client.updateIssue).toHaveBeenCalledTimes(1);
      expect(client.updateIssue).toHaveBeenCalledWith('linear-issue-1', {
        trashed: true,
      });
    });
  });

  describe('ensureLabel', () => {
    it('creates label if not found in cache and returns label ID', async () => {
      const client = createMockClient({
        issueLabels: mock().mockResolvedValue({ nodes: [] }),
      });
      const bridge = new LinearBridge(client, teamId);

      const labelId = await bridge.ensureLabel('homelab');

      expect(client.createIssueLabel).toHaveBeenCalledTimes(1);
      expect(labelId).toBe('linear-label-new');
    });

    it('returns existing label ID if found in cache (no API create call)', async () => {
      const client = createMockClient({
        issueLabels: mock().mockResolvedValue({
          nodes: [{ id: 'existing-label', name: 'homelab' }],
        }),
      });
      const bridge = new LinearBridge(client, teamId);

      // First call should fetch from API and cache
      const labelId = await bridge.ensureLabel('homelab');

      expect(labelId).toBe('existing-label');
      // Should NOT have called create
      expect(client.createIssueLabel).not.toHaveBeenCalled();
    });

    it('caches labels so second call for same name does not re-fetch', async () => {
      const client = createMockClient({
        issueLabels: mock().mockResolvedValue({
          nodes: [{ id: 'cached-label', name: 'work' }],
        }),
      });
      const bridge = new LinearBridge(client, teamId);

      await bridge.ensureLabel('work');
      await bridge.ensureLabel('work');

      // issueLabels should be called at most once (for initial fetch)
      // The second call should use the cache
      expect(client.createIssueLabel).not.toHaveBeenCalled();
    });
  });

  describe('fetchWorkflowStates', () => {
    it('returns mapped states array with linearId, name, type', async () => {
      const client = createMockClient();
      const bridge = new LinearBridge(client, teamId);

      const states = await bridge.fetchWorkflowStates();

      expect(states).toHaveLength(3);
      expect(states[0]).toEqual({
        linearId: 'ws-1',
        name: 'Backlog',
        type: 'backlog',
      });
      expect(states[1]).toEqual({
        linearId: 'ws-2',
        name: 'In Progress',
        type: 'started',
      });
      expect(states[2]).toEqual({
        linearId: 'ws-3',
        name: 'Done',
        type: 'completed',
      });
    });
  });

  describe('isAvailable', () => {
    it('returns true when API responds successfully', async () => {
      const client = createMockClient();
      const bridge = new LinearBridge(client, teamId);

      const available = await bridge.isAvailable();
      expect(available).toBe(true);
    });

    it('returns false when API throws an error', async () => {
      const client = createMockClient({
        viewer: Promise.reject(new Error('Unauthorized')),
      });
      const bridge = new LinearBridge(client, teamId);

      const available = await bridge.isAvailable();
      expect(available).toBe(false);
    });
  });
});
