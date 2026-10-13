package deployments

import (
	"strings"
	"testing"
)

const nodeTemplateFixture = `FROM node:20-alpine AS builder
COPY package*.json ./
RUN npm ci --ignore-scripts
RUN npm run build
FROM node:20-alpine
COPY package*.json ./
RUN npm ci --ignore-scripts --omit=dev
CMD ["npm", "start"]
`

func TestRenderNodePackageManagerTemplatePNPM(t *testing.T) {
	got := renderNodePackageManagerTemplate(nodeTemplateFixture, pkgManagerPNPM)
	for _, want := range []string{
		"COPY package.json pnpm-lock.yaml ./",
		"RUN corepack enable\nRUN pnpm install --frozen-lockfile --ignore-scripts",
		"RUN corepack enable\nRUN pnpm install --frozen-lockfile --ignore-scripts --prod",
		"RUN pnpm run build",
		`CMD ["pnpm", "start"]`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("pnpm template missing %q:\n%s", want, got)
		}
	}
}

func TestRenderNodePackageManagerTemplateYarn(t *testing.T) {
	got := renderNodePackageManagerTemplate(nodeTemplateFixture, pkgManagerYarn)
	for _, want := range []string{
		"COPY package.json yarn.lock ./",
		"RUN corepack enable\nRUN yarn install --frozen-lockfile --ignore-scripts",
		"RUN corepack enable\nRUN yarn install --frozen-lockfile --ignore-scripts --production=true",
		"RUN yarn run build",
		`CMD ["yarn", "start"]`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("yarn template missing %q:\n%s", want, got)
		}
	}
}
