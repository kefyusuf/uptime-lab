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
  } else throw new Error('Unknown browser phase.');
  expect(errors).toEqual([]);
});
