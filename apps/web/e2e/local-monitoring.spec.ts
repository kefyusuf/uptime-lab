import { expect, test } from '@playwright/test';
import { readFileSync, writeFileSync } from 'node:fs';
const artifact = process.env.WEB_BROWSER_ARTIFACT;
if (!artifact) throw new Error('Task-owned browser artifact is required.');
const target = 'http://web/';
test('built local monitor journey', async ({ page, request }) => {
  const errors: string[] = [];
  page.on('pageerror', (error) => errors.push(error.message));
  if (process.env.WEB_BROWSER_PHASE === 'register') {
    await page.goto('/');
    await page.getByLabel('Target URL').fill(target);
    await page.getByLabel('Target URL').press('Enter');
    await expect(
      page.getByRole('heading', { name: 'Monitor detail' }),
    ).toBeFocused();
    await expect(page.getByTestId('monitor-target')).toHaveText(target);
    const id = await page.getByTestId('monitor-id').textContent();
    expect(id).toMatch(/^[0-9a-f-]{36}$/);
    await expect(page.getByTestId('availability-status')).toHaveText('Unknown');
    await expect(page.getByTestId('availability-reason')).toHaveAttribute(
      'data-reason',
      'no_result',
    );
    await expect(page.getByText('No completed result yet')).toHaveCount(2);
    writeFileSync(artifact, JSON.stringify({ id, target }));
    const bg = await page
      .locator('.panel')
      .first()
      .evaluate((element) => getComputedStyle(element).backgroundColor);
    expect(bg).toBe('rgb(255, 255, 255)');
    await page.goBack();
    await expect(
      page.getByRole('heading', { name: 'Create monitor' }),
    ).toBeVisible();
    await page.goForward();
    await expect(page.getByTestId('monitor-id')).toHaveText(id!);
    await page.reload();
    await expect(page.getByTestId('monitor-id')).toHaveText(id!);
    await page.setViewportSize({ width: 390, height: 844 });
    const cards = page.locator('.read-card');
    await expect(cards).toHaveCount(2);
    const first = await cards.nth(0).boundingBox(),
      second = await cards.nth(1).boundingBox();
    expect(second!.y).toBeGreaterThan(first!.y + first!.height);
    for (const [path, status, options] of [
      ['/api/internal/checks/claim', 404, {}],
      ['/api/monitors/%2f', 400, {}],
      [
        '/api/monitors',
        403,
        {
          headers: { Origin: 'http://foreign.invalid' },
          data: { targetUrl: target },
        },
      ],
      [
        '/api/monitors',
        415,
        {
          headers: {
            Origin: new URL(page.url()).origin,
            'Content-Type': 'application/x-www-form-urlencoded',
          },
          data: 'targetUrl=http://web/',
        },
      ],
    ] as const) {
      const response = await request.fetch(path, {
        method: path === '/api/monitors' ? 'POST' : 'GET',
        ...options,
      });
      expect(response.status()).toBe(status);
    }
    const wrongHost = await request.get('/', {
      headers: { Host: 'foreign.invalid' },
    });
    expect(wrongHost.status()).toBe(403);
    const registeredIds = [id!];
    for (let index = 0; index < 20; index++) {
      const response = await request.post('/api/monitors', {
        headers: { Origin: new URL(page.url()).origin },
        data: { targetUrl: target },
      });
      expect(response.status()).toBe(201);
      const monitor = (await response.json()) as {
        id: string;
        targetUrl: string;
      };
      expect(monitor.targetUrl).toBe(target);
      expect(monitor.id).toMatch(/^[0-9a-f-]{36}$/);
      registeredIds.push(monitor.id);
    }
    // Discover the registered inventory through a fresh start-page load.
    await page.goto('/');
    await page.reload();
    const inventoryRows = page.getByTestId('inventory-row');
    await expect(inventoryRows).toHaveCount(20);
    await expect(inventoryRows.locator('.inventory-target')).toHaveText(
      Array<string>(20).fill(target),
    );
    expect(
      await page.evaluate(
        () => document.documentElement.scrollWidth <= window.innerWidth,
      ),
    ).toBe(true);
    const ids = await inventoryRows
      .locator('a')
      .evaluateAll((links) =>
        links.map((link) => link.getAttribute('href')!.split('/').at(-1)!),
      );
    await page.getByRole('button', { name: 'Next page' }).click();
    await expect(inventoryRows).toHaveCount(1);
    const selected = inventoryRows.getByRole('link');
    const selectedId = (await selected.getAttribute('href'))!
      .split('/')
      .at(-1)!;
    ids.push(selectedId);
    expect(new Set(ids).size).toBe(21);
    expect(new Set(ids)).toEqual(new Set(registeredIds));
    await selected.focus();
    await selected.press('Enter');
    await expect(page.getByTestId('monitor-id')).toHaveText(selectedId);
    await expect(page.getByTestId('monitor-target')).toHaveText(target);
    await expect(page.getByTestId('availability-reason')).toHaveAttribute(
      'data-reason',
      'no_result',
    );
    await expect(page.getByText('No completed result yet')).toHaveCount(2);
    writeFileSync(
      artifact,
      JSON.stringify({ id, target, inventory: { ids, selectedId } }),
    );
  } else if (process.env.WEB_BROWSER_PHASE === 'result') {
    const saved = JSON.parse(readFileSync(artifact, 'utf8')) as {
      id: string;
      target: string;
    };
    expect(saved.target).toBe(target);
    await page.goto('/monitors/' + saved.id);
    await expect(page.getByTestId('monitor-id')).toHaveText(saved.id);
    await expect(async () => {
      await page.getByRole('button', { name: 'Refresh monitor' }).click();
      await expect(page.getByTestId('raw-result-kind')).toHaveText(
        'policy_rejected',
        { timeout: 3000 },
      );
      await expect(page.getByTestId('availability-reason')).toHaveAttribute(
        'data-reason',
        'policy_rejected',
        { timeout: 3000 },
      );
      await expect(page.getByTestId('availability-status')).toHaveText(
        'Unknown',
      );
    }).toPass({ timeout: 60000, intervals: [1000, 2000] });
    const capture = {
      ...saved,
      raw: {
        checkId: await page.getByTestId('raw-check-id').textContent(),
        completedAt: await page
          .getByTestId('raw-completed-at')
          .getAttribute('datetime'),
        durationMs: Number(
          (await page.getByTestId('raw-duration').textContent())!.replace(
            ' ms',
            '',
          ),
        ),
      },
      availability: {
        checkId: await page.getByTestId('availability-check-id').textContent(),
        completedAt: await page
          .getByTestId('availability-completed-at')
          .getAttribute('datetime'),
      },
    };
    writeFileSync(artifact, JSON.stringify(capture));
  } else if (
    ['pause', 'paused', 'resume'].includes(process.env.WEB_BROWSER_PHASE || '')
  ) {
    const saved = JSON.parse(readFileSync(artifact, 'utf8')) as {
      id: string;
      scheduling?: Record<string, unknown>;
    };
    const phase = process.env.WEB_BROWSER_PHASE;
    if (phase === 'pause') {
      await page.goto('/');
      await expect(page.getByTestId('inventory-row')).toHaveCount(20);
      const link = page.locator('a[href="/monitors/' + saved.id + '"]');
      if ((await link.count()) === 0)
        await page.getByRole('button', { name: 'Next page' }).click();
      await expect(link).toBeVisible();
      await link.focus();
      await link.press('Enter');
    } else await page.goto('/monitors/' + saved.id);
    await expect(page.getByTestId('monitor-id')).toHaveText(saved.id);
    if (phase === 'pause') {
      await expect(page.getByTestId('scheduling-state')).toHaveText('Active');
      await page.getByRole('button', { name: 'Pause', exact: true }).click();
      await expect(page.getByTestId('scheduling-state')).toHaveText('Paused');
      await page.reload();
      await expect(page.getByTestId('scheduling-state')).toHaveText('Paused');
      saved.scheduling = { monitorId: saved.id, pausedState: 'paused' };
    } else if (phase === 'paused') {
      await expect(page.getByTestId('scheduling-state')).toHaveText('Paused');
      await expect(
        page.getByRole('button', { name: 'Resume', exact: true }),
      ).toBeEnabled();
    } else {
      await expect(page.getByTestId('scheduling-state')).toHaveText('Paused');
      await page.getByRole('button', { name: 'Resume', exact: true }).click();
      await expect(page.getByTestId('scheduling-state')).toHaveText('Active');
      await page.reload();
      await expect(page.getByTestId('scheduling-state')).toHaveText('Active');
      saved.scheduling!.resumedState = 'active';
    }
    writeFileSync(artifact, JSON.stringify(saved));
  } else if (
    ['delete-register', 'delete'].includes(process.env.WEB_BROWSER_PHASE || '')
  ) {
    const saved = JSON.parse(readFileSync(artifact, 'utf8')) as {
      id: string;
      deletion?: Record<string, unknown>;
    };
    const phase = process.env.WEB_BROWSER_PHASE;
    if (phase === 'delete-register') {
      await page.goto('/');
      await page.getByLabel('Target URL').fill(target);
      await page.getByLabel('Target URL').press('Enter');
      await expect(
        page.getByRole('heading', { name: 'Monitor detail' }),
      ).toBeFocused();
      const id = await page.getByTestId('monitor-id').textContent();
      expect(id).toMatch(/^[0-9a-f-]{36}$/);
      expect(id).not.toBe(saved.id);
      await expect(page.getByTestId('scheduling-state')).toHaveText('Active');
      const deleteButton = page.getByRole('button', { name: 'Delete monitor' });
      await expect(deleteButton).toBeEnabled();
      await deleteButton.click();
      await expect(
        page.getByRole('button', { name: 'Delete permanently' }),
      ).toBeVisible();
      await page.getByRole('button', { name: 'Cancel' }).click();
      await expect(deleteButton).toBeEnabled();
      await expect(page.getByTestId('monitor-id')).toHaveText(id!);
      writeFileSync(
        artifact,
        JSON.stringify({ ...saved, deletion: { monitorId: id } }),
      );
    } else {
      const deletionMonitorId = (saved.deletion as { monitorId: string })
        .monitorId;
      await page.goto('/monitors/' + deletionMonitorId);
      await expect(page.getByTestId('monitor-id')).toHaveText(
        deletionMonitorId,
      );
      await page.getByRole('button', { name: 'Delete monitor' }).click();
      await page.getByRole('button', { name: 'Delete permanently' }).click();
      await page.waitForURL('/');
      const inventoryRows = page.getByTestId('inventory-row');
      await expect(inventoryRows.first()).toBeVisible();
      await expect(
        page.locator('a[href="/monitors/' + deletionMonitorId + '"]'),
      ).toHaveCount(0);
      const gatewayRead = await request.get(
        '/api/monitors/' + deletionMonitorId,
      );
      expect(gatewayRead.status()).toBe(404);
      expect(gatewayRead.headers()['cache-control']).toBe('no-store');
      saved.deletion!.deletedState = 'deleted';
      saved.deletion!.inventoryExcludes = true;
      writeFileSync(artifact, JSON.stringify(saved));
    }
  } else if (process.env.WEB_BROWSER_PHASE === 'history') {
    const saved = JSON.parse(readFileSync(artifact, 'utf8')) as {
      history?: Record<string, unknown>;
    };
    const historyMonitorId = (saved.history as { monitorId: string }).monitorId;
    await page.goto('/monitors/' + historyMonitorId);
    await expect(page.getByTestId('history-list')).toBeVisible();
    const firstIds = await page
      .getByTestId('history-entry')
      .evaluateAll((elements) =>
        elements.map((element) => element.getAttribute('data-check-id')),
      );
    const times = await page
      .getByTestId('history-entry')
      .evaluateAll((elements) =>
        elements.map((element) =>
          element.querySelector('time')?.getAttribute('datetime'),
        ),
      );
    const instants = times.map((value) => new Date(value!).getTime());
    const orderConfirmed = instants.every(
      (value, index) => index === 0 || value <= instants[index - 1],
    );
    await page.getByRole('button', { name: 'Next history page' }).click();
    await page.waitForFunction(
      (previous) =>
        document
          .querySelector('[data-testid="history-entry"]')
          ?.getAttribute('data-check-id') !== previous,
      firstIds[0],
    );
    const secondIds = await page
      .getByTestId('history-entry')
      .evaluateAll((elements) =>
        elements.map((element) => element.getAttribute('data-check-id')),
      );
    writeFileSync(
      artifact,
      JSON.stringify({
        ...saved,
        history: {
          ...saved.history,
          firstPageIds: firstIds,
          secondPageIds: secondIds,
          orderConfirmed,
        },
      }),
    );
  } else if (
    ['edit-register', 'edit'].includes(process.env.WEB_BROWSER_PHASE || '')
  ) {
    const saved = JSON.parse(readFileSync(artifact, 'utf8')) as {
      id: string;
      targetEdit?: Record<string, unknown>;
    };
    const phase = process.env.WEB_BROWSER_PHASE;
    if (phase === 'edit-register') {
      await page.goto('/');
      await page.getByLabel('Target URL').fill(target);
      await page.getByLabel('Target URL').press('Enter');
      await expect(
        page.getByRole('heading', { name: 'Monitor detail' }),
      ).toBeFocused();
      const id = await page.getByTestId('monitor-id').textContent();
      expect(id).toMatch(/^[0-9a-f-]{36}$/);
      expect(id).not.toBe(saved.id);
      await expect(page.getByTestId('scheduling-state')).toHaveText('Active');
      await expect(page.getByTestId('monitor-target')).toHaveText(target);
      writeFileSync(
        artifact,
        JSON.stringify({
          ...saved,
          targetEdit: { monitorId: id, oldTarget: target },
        }),
      );
    } else {
      const edit = saved.targetEdit as { monitorId: string; oldTarget: string };
      await page.goto('/monitors/' + edit.monitorId);
      await expect(page.getByTestId('monitor-id')).toHaveText(edit.monitorId);
      const editButton = page.getByRole('button', { name: 'Edit target' });
      await expect(editButton).toBeEnabled();
      await editButton.click();
      const input = page.getByLabel('Target URL');
      await expect(input).toHaveValue(edit.oldTarget);
      await input.fill('http://web/edited');
      await page.getByRole('button', { name: 'Save target' }).click();
      await expect(page.getByText('Target updated.')).toBeVisible();
      await expect(page.getByTestId('monitor-target')).toHaveText(
        'http://web/edited',
      );
      writeFileSync(
        artifact,
        JSON.stringify({
          ...saved,
          targetEdit: { ...edit, newTarget: 'http://web/edited' },
        }),
      );
    }
  } else throw new Error('Unknown browser phase.');
  expect(errors).toEqual([]);
});
