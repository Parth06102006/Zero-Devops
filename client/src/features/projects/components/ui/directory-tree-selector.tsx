"use client";

import React, { useState, useMemo, useEffect } from "react";
import {
  Folder,
  FolderOpen,
  ChevronRight,
  Check,
  Search,
  FolderTree,
  Home,
  X,
  SlidersHorizontal,
  Loader2,
} from "lucide-react";
import { cn } from "@/lib/utils/cn";
import { Input } from "@/components/ui/input";
import { Button } from "@/components/ui/button";

export interface DirectoryNode {
  name: string;
  path: string;
  children: DirectoryNode[];
}

export function buildDirectoryTree(paths: string[]): DirectoryNode[] {
  const rootNodes: DirectoryNode[] = [];
  const nodeMap = new Map<string, DirectoryNode>();

  const sortedPaths = Array.from(new Set(paths))
    .filter((p) => p && p !== "." && p.trim() !== "")
    .sort();

  for (const fullPath of sortedPaths) {
    const parts = fullPath.split("/").filter(Boolean);
    let currentPath = "";

    for (let i = 0; i < parts.length; i++) {
      const part = parts[i];
      if (!part) continue;
      const parentPath = currentPath;
      currentPath = currentPath ? `${currentPath}/${part}` : part;

      if (!nodeMap.has(currentPath)) {
        const newNode: DirectoryNode = {
          name: part,
          path: currentPath,
          children: [],
        };
        nodeMap.set(currentPath, newNode);

        if (parentPath && nodeMap.has(parentPath)) {
          nodeMap.get(parentPath)!.children.push(newNode);
        } else if (i === 0) {
          rootNodes.push(newNode);
        }
      }
    }
  }

  return rootNodes;
}

interface TreeNodeProps {
  node: DirectoryNode;
  selectedPath: string;
  onSelect: (path: string) => void;
  expandedPaths: Set<string>;
  onToggleExpand: (path: string) => void;
  searchFilter: string;
  depth?: number;
}

function TreeNode({
  node,
  selectedPath,
  onSelect,
  expandedPaths,
  onToggleExpand,
  searchFilter,
  depth = 0,
}: TreeNodeProps) {
  const isSelected = selectedPath === node.path;
  const isExpanded = expandedPaths.has(node.path);
  const hasChildren = node.children.length > 0;

  // If search filter is active, check if this node or any descendants match
  const matchesFilter = (n: DirectoryNode): boolean => {
    if (!searchFilter.trim()) return true;
    if (n.name.toLowerCase().includes(searchFilter.toLowerCase())) return true;
    return n.children.some(matchesFilter);
  };

  if (searchFilter && !matchesFilter(node)) {
    return null;
  }

  return (
    <div className="select-none">
      <div
        className={cn(
          "group relative flex items-center gap-1.5 rounded-lg px-2 py-1.5 text-xs transition-colors cursor-pointer",
          isSelected
            ? "bg-cyan-500/15 text-cyan-200 border border-cyan-400/30 font-medium"
            : "text-white/70 hover:bg-white/[0.04] hover:text-white"
        )}
        style={{ paddingLeft: `${depth * 18 + 8}px` }}
        onClick={() => onSelect(node.path)}
      >
        {/* Branch guideline for nested levels */}
        {depth > 0 && (
          <span
            className="pointer-events-none absolute left-0 top-0 bottom-0 border-l border-white/[0.07]"
            style={{ left: `${depth * 18 - 4}px` }}
          />
        )}

        {/* Expand/Collapse Chevron */}
        {hasChildren ? (
          <button
            type="button"
            className="flex size-5 shrink-0 items-center justify-center rounded p-0 text-white/40 hover:bg-white/10 hover:text-white"
            onClick={(e) => {
              e.stopPropagation();
              onToggleExpand(node.path);
            }}
            aria-label={isExpanded ? "Collapse" : "Expand"}
          >
            <ChevronRight
              className={cn("size-3.5 transition-transform duration-150", isExpanded && "rotate-90")}
            />
          </button>
        ) : (
          <span className="size-5 shrink-0" />
        )}

        {/* Folder Icon */}
        <span className="shrink-0">
          {isExpanded || isSelected ? (
            <FolderOpen className={cn("size-4", isSelected ? "text-cyan-300" : "text-amber-400/80")} />
          ) : (
            <Folder className="size-4 text-white/40 group-hover:text-amber-300/70" />
          )}
        </span>

        {/* Node Name */}
        <span className="font-mono truncate">{node.name}</span>

        {/* Child count badge if collapsed */}
        {hasChildren && !isExpanded && (
          <span className="ml-1 rounded bg-white/[0.05] px-1.5 py-0.2 text-[9px] text-white/35">
            {node.children.length}
          </span>
        )}

        {/* Selected Checkmark */}
        {isSelected && (
          <Check className="size-3.5 shrink-0 text-cyan-300 ml-auto" />
        )}
      </div>

      {/* Children */}
      {hasChildren && isExpanded && (
        <div className="relative">
          {node.children.map((child) => (
            <TreeNode
              key={child.path}
              node={child}
              selectedPath={selectedPath}
              onSelect={onSelect}
              expandedPaths={expandedPaths}
              onToggleExpand={onToggleExpand}
              searchFilter={searchFilter}
              depth={depth + 1}
            />
          ))}
        </div>
      )}
    </div>
  );
}

export interface DirectoryTreeSelectorProps {
  directories: string[];
  selectedPath: string;
  onSelect: (path: string) => void;
  isLoading?: boolean;
  className?: string;
}

export function DirectoryTreeSelector({
  directories,
  selectedPath,
  onSelect,
  isLoading = false,
  className,
}: DirectoryTreeSelectorProps) {
  const [search, setSearch] = useState("");
  const [expandedPaths, setExpandedPaths] = useState<Set<string>>(new Set());
  const [customInputOpen, setCustomInputOpen] = useState(false);

  const tree = useMemo(() => buildDirectoryTree(directories), [directories]);

  // Auto-expand paths leading up to the selected path
  useEffect(() => {
    if (selectedPath && selectedPath !== ".") {
      const parts = selectedPath.split("/");
      const toExpand = new Set(expandedPaths);
      let cur = "";
      for (let i = 0; i < parts.length - 1; i++) {
        const part = parts[i];
        if (!part) continue;
        cur = cur ? `${cur}/${part}` : part;
        toExpand.add(cur);
      }
      setExpandedPaths(toExpand);
    }
  }, [selectedPath]);

  // When searching, auto-expand all nodes
  useEffect(() => {
    if (search.trim()) {
      const allPaths = new Set<string>();
      const collect = (nodes: DirectoryNode[]) => {
        for (const n of nodes) {
          allPaths.add(n.path);
          if (n.children.length > 0) collect(n.children);
        }
      };
      collect(tree);
      setExpandedPaths(allPaths);
    }
  }, [search, tree]);

  const toggleExpand = (path: string) => {
    setExpandedPaths((prev) => {
      const next = new Set(prev);
      if (next.has(path)) {
        next.delete(path);
      } else {
        next.add(path);
      }
      return next;
    });
  };

  const expandAll = () => {
    const all = new Set<string>();
    const collect = (nodes: DirectoryNode[]) => {
      for (const n of nodes) {
        all.add(n.path);
        if (n.children.length > 0) collect(n.children);
      }
    };
    collect(tree);
    setExpandedPaths(all);
  };

  const collapseAll = () => {
    setExpandedPaths(new Set());
  };

  // Compute breadcrumbs
  const breadcrumbs = useMemo(() => {
    if (!selectedPath || selectedPath === ".") {
      return [{ name: "root", path: "." }];
    }
    const parts = selectedPath.split("/");
    const crumbs = [{ name: "root", path: "." }];
    let acc = "";
    for (const part of parts) {
      acc = acc ? `${acc}/${part}` : part;
      crumbs.push({ name: part, path: acc });
    }
    return crumbs;
  }, [selectedPath]);

  const isRootSelected = !selectedPath || selectedPath === ".";

  return (
    <div className={cn("space-y-3 rounded-xl border border-white/[0.08] bg-[#0c0c0e] p-3.5 shadow-xl", className)}>
      {/* Header with active selection & breadcrumb trail */}
      <div className="flex flex-wrap items-center justify-between gap-2 border-b border-white/[0.06] pb-3">
        <div className="flex items-center gap-2 min-w-0">
          <span className="flex size-7 shrink-0 items-center justify-center rounded-lg bg-cyan-400/10 text-cyan-300">
            <FolderTree className="size-4" />
          </span>
          <div className="min-w-0">
            <div className="flex items-center gap-1.5 flex-wrap text-xs">
              <span className="text-white/40">Selected:</span>
              <div className="flex items-center gap-1 font-mono text-[11px]">
                {breadcrumbs.map((crumb, idx) => (
                  <React.Fragment key={crumb.path}>
                    {idx > 0 && <span className="text-white/20">/</span>}
                    <button
                      type="button"
                      onClick={() => onSelect(crumb.path)}
                      className={cn(
                        "rounded px-1.5 py-0.5 transition-colors",
                        crumb.path === selectedPath || (crumb.path === "." && isRootSelected)
                          ? "bg-cyan-400/20 text-cyan-300 font-semibold border border-cyan-400/30"
                          : "text-white/50 hover:bg-white/[0.06] hover:text-white"
                      )}
                    >
                      {crumb.name}
                    </button>
                  </React.Fragment>
                ))}
              </div>
            </div>
          </div>
        </div>

        {/* Tree controls: Expand / Collapse all & Custom path toggle */}
        <div className="flex items-center gap-1.5">
          <Button
            type="button"
            variant="ghost"
            size="sm"
            onClick={expandAll}
            className="h-7 px-2 text-[10px] text-white/40 hover:text-white"
          >
            Expand all
          </Button>
          <Button
            type="button"
            variant="ghost"
            size="sm"
            onClick={collapseAll}
            className="h-7 px-2 text-[10px] text-white/40 hover:text-white"
          >
            Collapse
          </Button>
          <Button
            type="button"
            variant="outline"
            size="sm"
            onClick={() => setCustomInputOpen(!customInputOpen)}
            className={cn(
              "h-7 gap-1 px-2 text-[10px]",
              customInputOpen && "border-cyan-400/30 bg-cyan-400/10 text-cyan-300"
            )}
          >
            <SlidersHorizontal className="size-3" />
            <span>Custom path</span>
          </Button>
        </div>
      </div>

      {/* Manual custom path input if opened */}
      {customInputOpen && (
        <div className="flex items-center gap-2 rounded-lg bg-white/[0.02] p-2 border border-white/[0.05]">
          <span className="text-[11px] font-medium text-white/50">Path:</span>
          <Input
            value={selectedPath}
            onChange={(e) => onSelect(e.target.value)}
            placeholder="e.g. apps/web or packages/ui"
            className="h-7 border-white/10 bg-black/40 font-mono text-xs text-white placeholder:text-white/20"
          />
          <Button
            type="button"
            variant="ghost"
            size="sm"
            onClick={() => onSelect(".")}
            className="h-7 px-2 text-[10px] text-white/40 hover:text-white"
          >
            Reset to Root
          </Button>
        </div>
      )}

      {/* Search Input */}
      <div className="relative">
        <Search className="pointer-events-none absolute left-2.5 top-2.5 size-3.5 text-white/30" />
        <Input
          value={search}
          onChange={(e) => setSearch(e.target.value)}
          placeholder="Filter modules & folders in tree…"
          className="h-8 border-white/10 bg-white/[0.02] pl-8 pr-7 text-xs text-white placeholder:text-white/25 focus:border-cyan-400/40"
        />
        {search && (
          <button
            type="button"
            onClick={() => setSearch("")}
            className="absolute right-2.5 top-2.5 text-white/30 hover:text-white"
          >
            <X className="size-3.5" />
          </button>
        )}
      </div>

      {/* Tree Content Area */}
      <div className="max-h-[220px] overflow-y-auto space-y-0.5 rounded-lg border border-white/[0.04] bg-black/20 p-1.5">
        {isLoading ? (
          <div className="flex items-center justify-center gap-2 py-6 text-xs text-white/40">
            <Loader2 className="size-4 animate-spin text-cyan-400" />
            <span>Scanning repository tree…</span>
          </div>
        ) : (
          <>
            {/* Root item */}
            <div
              onClick={() => onSelect(".")}
              className={cn(
                "group flex items-center justify-between rounded-lg px-2.5 py-1.5 text-xs transition-colors cursor-pointer",
                isRootSelected
                  ? "bg-cyan-500/15 text-cyan-200 border border-cyan-400/30 font-medium"
                  : "text-white/80 hover:bg-white/[0.04] hover:text-white"
              )}
            >
              <div className="flex items-center gap-2 min-w-0">
                <span className="flex size-5 shrink-0 items-center justify-center rounded bg-emerald-400/15 text-emerald-300">
                  <Home className="size-3" />
                </span>
                <span className="font-mono truncate">. (Repository Root)</span>
                <span className="rounded bg-white/[0.06] px-1.5 py-0.2 text-[9px] text-white/40">
                  Default
                </span>
              </div>
              {isRootSelected && <Check className="size-3.5 shrink-0 text-cyan-300" />}
            </div>

            {/* Tree nodes */}
            {tree.map((node) => (
              <TreeNode
                key={node.path}
                node={node}
                selectedPath={selectedPath}
                onSelect={onSelect}
                expandedPaths={expandedPaths}
                onToggleExpand={toggleExpand}
                searchFilter={search}
              />
            ))}

            {tree.length === 0 && !isLoading && (
              <div className="py-4 text-center text-xs text-white/30">
                No submodules or subfolders detected. Root directory will be deployed.
              </div>
            )}
          </>
        )}
      </div>

      {/* Submodule path filtering hint */}
      <p className="text-[10px] text-white/40">
        💡 <strong className="text-white/60">Submodule isolation:</strong> Webhooks will build this project only when files in <code className="text-cyan-300 font-mono">{selectedPath || "."}</code> or root configuration files are modified.
      </p>
    </div>
  );
}
