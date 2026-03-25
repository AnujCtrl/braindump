// src/core/linear.ts
//
// LinearBridge — adapter between the local store and the Linear API.
// Wraps a Linear SDK client (or any compatible mock) to isolate all
// Linear-specific API calls behind a stable interface.

export class LinearBridge {
  private client: any;
  private teamId: string;
  private labelCache: Map<string, string>;

  constructor(client: any, teamId: string) {
    this.client = client;
    this.teamId = teamId;
    this.labelCache = new Map();
  }

  async createIssue(opts: {
    title: string;
    description: string;
    priority: number;
    stateId?: string;
    labelIds?: string[];
  }): Promise<string> {
    const result = await this.client.issueCreate({
      teamId: this.teamId,
      title: opts.title,
      description: opts.description,
      priority: opts.priority,
      stateId: opts.stateId,
      labelIds: opts.labelIds,
    });
    const issue = await result.issue;
    return issue.id;
  }

  async updateIssue(linearId: string, fields: Record<string, unknown>): Promise<void> {
    await this.client.issueUpdate(linearId, fields);
  }

  async deleteIssue(linearId: string): Promise<void> {
    await this.client.issueUpdate(linearId, { trashed: true });
  }

  async ensureLabel(name: string): Promise<string> {
    if (this.labelCache.has(name)) {
      return this.labelCache.get(name)!;
    }

    const result = await this.client.issueLabels();
    const existing = result.nodes.find((n: { id: string; name: string }) => n.name === name);
    if (existing) {
      this.labelCache.set(name, existing.id);
      return existing.id;
    }

    const created = await this.client.issueLabelCreate({ teamId: this.teamId, name });
    const label = await created.issueLabel;
    this.labelCache.set(name, label.id);
    return label.id;
  }

  async fetchWorkflowStates(): Promise<Array<{ linearId: string; name: string; type: string }>> {
    const result = await this.client.workflowStates({
      filter: { team: { id: { eq: this.teamId } } },
    });
    return result.nodes.map((node: { id: string; name: string; type: string }) => ({
      linearId: node.id,
      name: node.name,
      type: node.type,
    }));
  }

  async isAvailable(): Promise<boolean> {
    try {
      await this.client.viewer;
      return true;
    } catch {
      return false;
    }
  }
}
