"use client";

import { useState } from "react";
import { Plus, Trash2, Eye, EyeOff, Copy } from "lucide-react";
import { Input } from "@/components/ui/input";
import { Button } from "@/components/ui/button";

interface EnvVar {
  key: string;
  value: string;
  isSecret: boolean;
}

interface EnvVarTableProps {
  variables: EnvVar[];
  onChange: (variables: EnvVar[]) => void;
  disabled?: boolean;
  placeholderKey?: string;
  placeholderValue?: string;
}

export function EnvVarTable({ 
  variables, 
  onChange, 
  disabled = false,
  placeholderKey = "NEXT_PUBLIC_API_URL",
  placeholderValue = "https://api.example.com",
}: EnvVarTableProps) {
  const [editingIndex, setEditingIndex] = useState<number | null>(null);
  const [showValues, setShowValues] = useState<Record<number, boolean>>({});

  const handleKeyChange = (index: number, key: string) => {
    const newVars = [...variables];
    const current = newVars[index];
    if (current) {
      newVars[index] = { ...current, key };
      onChange(newVars);
    }
  };

  const handleValueChange = (index: number, value: string) => {
    const newVars = [...variables];
    const current = newVars[index];
    if (current) {
      newVars[index] = { ...current, value };
      onChange(newVars);
    }
  };

  const handleAdd = () => {
    onChange([...variables, { key: "", value: "", isSecret: true }]);
    setEditingIndex(variables.length);
  };

  const handleRemove = (index: number) => {
    onChange(variables.filter((_, i) => i !== index));
  };

  const handleToggleSecret = (index: number) => {
    const newVars = [...variables];
    const current = newVars[index];
    if (current) {
      newVars[index] = { ...current, isSecret: !current.isSecret };
      onChange(newVars);
    }
  };

  const handleCopy = (value: string) => {
    navigator.clipboard.writeText(value);
  };

  return (
    <div className="space-y-3">
      <div className="flex items-center justify-between">
        <h4 className="text-sm font-medium text-white/60">Environment Variables</h4>
        <Button
          variant="outline"
          size="sm"
          onClick={handleAdd}
          disabled={disabled}
          className="gap-1.5"
        >
          <Plus className="size-3.5" /> Add
        </Button>
      </div>

      {variables.length === 0 && !disabled && (
        <div className="rounded-xl border border-dashed border-white/10 p-8 text-center">
          <p className="text-sm text-white/50">No environment variables configured.</p>
          <p className="mt-1 text-xs text-white/30">Add variables for your build and runtime.</p>
        </div>
      )}

      <div className="rounded-xl border border-white/[0.08] bg-white/[0.02] overflow-hidden">
        <div className="grid grid-cols-[1fr_2fr_auto] border-b border-white/[0.08] px-3 py-2 text-[10px] font-medium uppercase tracking-wider text-white/30">
          <span>Key</span>
          <span>Value</span>
          <span className="text-center">Actions</span>
        </div>

        {variables.map((variable, index) => (
          <div
            key={index}
            className="grid grid-cols-[1fr_2fr_auto] items-center gap-2 border-b border-white/[0.06] px-3 py-2 last:border-0"
          >
            <div>
              <Input
                value={variable.key}
                onChange={(e) => handleKeyChange(index, e.target.value)}
                placeholder={placeholderKey}
                disabled={disabled}
                className="bg-transparent border-0 focus:ring-0 text-sm text-white placeholder:text-white/25"
                autoFocus={editingIndex === index}
              />
            </div>
            <div className="relative">
              <Input
                type={variable.isSecret && !showValues[index] ? "password" : "text"}
                value={variable.value}
                onChange={(e) => handleValueChange(index, e.target.value)}
                placeholder={placeholderValue}
                disabled={disabled}
                className="bg-transparent border-0 focus:ring-0 text-sm text-white placeholder:text-white/25 pr-10"
              />
              {!disabled && (
                <button
                  type="button"
                  onClick={() => {
                    setShowValues((prev) => ({ ...prev, [index]: !prev[index] }));
                  }}
                  className="absolute right-2 top-1/2 -translate-y-1/2 text-white/30 hover:text-white"
                  aria-label={variable.isSecret ? "Show value" : "Hide value"}
                >
                  {variable.isSecret && !showValues[index] ? (
                    <EyeOff className="size-4" />
                  ) : (
                    <Eye className="size-4" />
                  )}
                </button>
              )}
            </div>
            <div className="flex items-center justify-end gap-1">
              {!disabled && (
                <Button
                  variant="ghost"
                  size="icon"
                  onClick={() => handleCopy(variable.value)}
                  className="text-white/30 hover:text-white"
                  aria-label="Copy value"
                >
                  <Copy className="size-3.5" />
                </Button>
              )}
              {!disabled && (
                <Button
                  variant="ghost"
                  size="icon"
                  onClick={() => handleToggleSecret(index)}
                  className="text-white/30 hover:text-white"
                  aria-label={variable.isSecret ? "Make public" : "Make secret"}
                >
                  {variable.isSecret ? (
                    <span className="size-3.5 rounded-full bg-amber-400/20 text-amber-300/80 flex items-center justify-center">
                      <span className="text-[8px] font-medium">🔒</span>
                    </span>
                  ) : (
                    <span className="size-3.5 rounded-full bg-emerald-400/20 text-emerald-300/80 flex items-center justify-center">
                      <span className="text-[8px] font-medium">🌐</span>
                    </span>
                  )}
                </Button>
              )}
              {!disabled && (
                <Button
                  variant="ghost"
                  size="icon"
                  onClick={() => handleRemove(index)}
                  className="text-white/30 hover:text-rose-300"
                  aria-label="Remove variable"
                >
                  <Trash2 className="size-3.5" />
                </Button>
              )}
            </div>
          </div>
        ))}

        {!disabled && variables.length > 0 && (
          <div className="p-3 border-t border-white/[0.06]">
            <p className="text-[10px] text-white/30">
              Secrets are encrypted at rest. Public variables are visible in build logs.
            </p>
          </div>
        )}
      </div>

      {disabled && variables.length === 0 && (
        <p className="text-xs text-white/30">Environment variables will be available after first deployment.</p>
      )}
    </div>
  );
}

export function EnvVarImport({ onImport }: { onImport: (vars: Record<string, string>) => void }) {
  const [content, setContent] = useState("");
  const [parsed, setParsed] = useState<Record<string, string>>({});

  const handleParse = () => {
    const lines = content.split("\n");
    const result: Record<string, string> = {};
    for (const line of lines) {
      const trimmed = line.trim();
      if (trimmed && !trimmed.startsWith("#")) {
        const idx = trimmed.indexOf("=");
        if (idx > 0) {
          const key = trimmed.slice(0, idx).trim();
          const value = trimmed.slice(idx + 1).trim();
          result[key] = value;
        }
      }
    }
    setParsed(result);
  };

  return (
    <div className="space-y-3">
      <div className="flex items-center gap-2">
        <Button variant="outline" size="sm" onClick={handleParse} disabled={!content.trim()}>
          Parse .env file
        </Button>
        {Object.keys(parsed).length > 0 && (
          <Button variant="default" size="sm" onClick={() => onImport(parsed)}>
            Import {Object.keys(parsed).length} variables
          </Button>
        )}
      </div>
      <textarea
        value={content}
        onChange={(e) => setContent(e.target.value)}
        placeholder="# Paste your .env file content here&#10;NEXT_PUBLIC_API_URL=https://api.example.com&#10;DATABASE_URL=postgresql://..."
        className="w-full min-h-[120px] rounded-lg border border-white/[0.1] bg-white/[0.03] px-3 py-2 text-sm text-white placeholder:text-white/25 outline-none focus:border-cyan-300/50 font-mono"
        rows={6}
      />
      {Object.keys(parsed).length > 0 && (
        <div className="rounded-lg border border-emerald-400/30 bg-emerald-400/5 p-3 text-xs text-emerald-200/80">
          Parsed {Object.keys(parsed).length} variables: {Object.keys(parsed).join(", ")}
        </div>
      )}
    </div>
  );
}