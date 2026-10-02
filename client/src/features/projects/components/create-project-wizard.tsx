"use client";

import React, { useState, useMemo, useCallback } from "react";
import { ChevronLeft, ChevronRight, Check, Loader2, Rocket, Github, Settings2, Key, Zap } from "lucide-react";
import { cn } from "@/lib/utils/cn";
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogDescription,
} from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { Button } from "@/components/ui/button";
import { Label } from "@/components/ui/label";
import { Separator } from "@/components/ui/separator";
import { FrameworkBadge, FrameworkSelector } from "./ui/framework-badge";
import { EnvVarTable, EnvVarImport } from "./ui/env-var-table";
import { detectFramework, FRAMEWORK_PRESETS, type FrameworkPreset } from "@/features/projects/lib/framework-detector";
import { useCreateProject } from "../hooks/use-projects";

interface SelectedRepo {
  id: number;
  full_name: string;
  name: string;
  owner: string;
  default_branch: string;
  clone_url: string;
  private: boolean;
}

interface CreateProjectWizardProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  selectedRepo?: SelectedRepo | null;
  onComplete?: (projectId: string) => void;
}

type WizardStep = "configure" | "env" | "deploy";

const STEPS: Array<{ id: WizardStep; title: string; icon: React.ComponentType<{ className?: string }>; description: string }> = [
  { id: "configure", title: "Configure", icon: Settings2, description: "Build settings & framework" },
  { id: "env", title: "Environment", icon: Key, description: "Environment variables" },
  { id: "deploy", title: "Deploy", icon: Rocket, description: "Review & create project" },
];

export function CreateProjectWizard({
  open,
  onOpenChange,
  selectedRepo,
  onComplete,
}: CreateProjectWizardProps) {
  const [step, setStep] = useState<WizardStep>("configure");
  const [framework, setFramework] = useState<string>("node");
  const [buildCommand, setBuildCommand] = useState("");
  const [outputDir, setOutputDir] = useState("");
  const [installCommand, setInstallCommand] = useState("");
  const [workingDir, setWorkingDir] = useState(".");
  const [branch, setBranch] = useState("");
  const [envVars, setEnvVars] = useState<Array<{ key: string; value: string; isSecret: boolean }>>([]);
  const [detectedFramework, setDetectedFramework] = useState<FrameworkPreset | null>(null);

  const create = useCreateProject();
  const [isDeploying, setIsDeploying] = useState(false);

  useMemo(() => {
    if (selectedRepo) {
      const detected = detectFramework(selectedRepo.name, null);
      if (detected) {
        setDetectedFramework(detected);
        setFramework(detected.name);
        setBuildCommand(detected.buildCommand);
        setOutputDir(detected.outputDirectory);
        setInstallCommand(detected.installCommand ?? "");
      } else {
        setFramework("node");
        setBuildCommand("npm run build");
        setOutputDir("dist");
        setInstallCommand("npm install");
      }
      setBranch(selectedRepo.default_branch);
    }
  }, [selectedRepo]);

  const currentStepIndex = STEPS.findIndex((s) => s.id === step);

  const goNext = useCallback(() => {
    if (currentStepIndex < STEPS.length - 1) {
      const nextStep = STEPS[currentStepIndex + 1];
      if (nextStep) setStep(nextStep.id);
    }
  }, [currentStepIndex]);

  const goBack = useCallback(() => {
    if (currentStepIndex > 0) {
      const prevStep = STEPS[currentStepIndex - 1];
      if (prevStep) setStep(prevStep.id);
    }
  }, [currentStepIndex]);

  const handleDeploy = async () => {
    if (!selectedRepo || !branch.trim() || !buildCommand.trim()) return;
    
    setIsDeploying(true);
    try {
      const result = await create.mutateAsync({
        repository_id: selectedRepo.id,
        configured_branch: branch.startsWith("refs/heads/") ? branch : `refs/heads/${branch}`,
        project_webhook_enabled: true,
        build_configuration: {
          executable: buildCommand.trim().split(/\s+/)[0] ?? "npm",
          args: buildCommand.trim().split(/\s+/).slice(1),
          working_dir: workingDir.trim() || ".",
          scanner_policy_version: "v1",
        },
      });
      onComplete?.(result.id);
      onOpenChange(false);
    } catch (error) {
      console.error("Failed to create project:", error);
    } finally {
      setIsDeploying(false);
    }
  };

  const preset = FRAMEWORK_PRESETS[framework] ?? FRAMEWORK_PRESETS.node;

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-w-2xl max-h-[90vh] border-white/10 bg-[#0a0a0a] text-white shadow-2xl overflow-hidden flex flex-col">
        <DialogHeader className="flex-shrink-0">
          <DialogTitle className="flex items-center gap-2 text-xl">
            <span className="flex size-8 items-center justify-center rounded-lg bg-gradient-to-br from-cyan-400/20 to-fuchsia-500/20">
              <Github className="size-4 text-cyan-200" />
            </span>
            {selectedRepo ? `Configure: ${selectedRepo.name}` : "New Project"}
          </DialogTitle>
          <DialogDescription className="text-white/45">
            {selectedRepo 
              ? `Repository: ${selectedRepo.full_name} • Branch: ${branch || selectedRepo.default_branch}`
              : "Select a repository to continue"}
          </DialogDescription>
        </DialogHeader>

        <div className="flex-shrink-0 px-6 py-4 border-b border-white/[0.07]">
          <div className="flex items-center gap-2">
            {STEPS.map((s, i) => (
              <React.Fragment key={s.id}>
                <Button
                  variant={i <= currentStepIndex ? "default" : "outline"}
                  size="sm"
                  className={cn(
                    "gap-1.5 transition-colors",
                    i < currentStepIndex && "bg-emerald-400/20 border-emerald-400/30 text-emerald-300",
                    i === currentStepIndex && "bg-cyan-400/20 border-cyan-400/30 text-cyan-300",
                    i > currentStepIndex && "text-white/30 hover:text-white/50",
                  )}
                  onClick={() => i <= currentStepIndex && setStep(s.id)}
                  disabled={i > currentStepIndex}
                >
                  {i < currentStepIndex ? <Check className="size-3.5" /> : <s.icon className="size-3.5" />}
                  <span className="hidden sm:inline">{s.title}</span>
                </Button>
                {i < STEPS.length - 1 && (
                  <div className={cn(
                    "flex-1 h-0.5 rounded",
                    i < currentStepIndex ? "bg-emerald-400/30" : "bg-white/[0.07]"
                  )} />
                )}
              </React.Fragment>
            ))}
          </div>
        </div>

        <div className="flex-1 overflow-y-auto p-6">
          {step === "configure" && (
            <div className="space-y-6">
              <div>
                <Label className="text-xs font-medium text-white/60">Framework Preset</Label>
                <FrameworkSelector value={framework} onChange={(v) => {
                  setFramework(v);
                  const p = FRAMEWORK_PRESETS[v];
                  if (p) {
                    setBuildCommand(p.buildCommand);
                    setOutputDir(p.outputDirectory);
                    setInstallCommand(p.installCommand ?? "");
                  }
                }} />
                {detectedFramework && detectedFramework.name !== framework && (
                  <p className="mt-2 text-xs text-amber-300/70">
                    Detected: <FrameworkBadge framework={detectedFramework.name} size="sm" />
                    — you can override if needed.
                  </p>
                )}
              </div>

              <Separator />

              <div className="grid gap-4 sm:grid-cols-2">
                <div>
                  <Label htmlFor="branch" className="text-xs font-medium text-white/60">Branch</Label>
                  <Input
                    id="branch"
                    value={branch}
                    onChange={(e) => setBranch(e.target.value)}
                    placeholder="main"
                    className="border-white/10 bg-white/[0.03] text-white placeholder:text-white/25"
                  />
                </div>
                <div>
                  <Label htmlFor="workingDir" className="text-xs font-medium text-white/60">Root Directory</Label>
                  <Input
                    id="workingDir"
                    value={workingDir}
                    onChange={(e) => setWorkingDir(e.target.value)}
                    placeholder="."
                    className="border-white/10 bg-white/[0.03] text-white placeholder:text-white/25"
                  />
                </div>
              </div>

              <div>
                <Label htmlFor="buildCommand" className="text-xs font-medium text-white/60">Build Command</Label>
                <Input
                  id="buildCommand"
                  value={buildCommand}
                  onChange={(e) => setBuildCommand(e.target.value)}
                  className="border-white/10 bg-white/[0.03] font-mono text-xs text-white"
                />
              </div>

              <div className="grid gap-4 sm:grid-cols-2">
                <div>
                  <Label htmlFor="outputDir" className="text-xs font-medium text-white/60">Output Directory</Label>
                  <Input
                    id="outputDir"
                    value={outputDir}
                    onChange={(e) => setOutputDir(e.target.value)}
                    placeholder=".next"
                    className="border-white/10 bg-white/[0.03] font-mono text-xs text-white"
                  />
                </div>
                <div>
                  <Label htmlFor="installCommand" className="text-xs font-medium text-white/60">Install Command</Label>
                  <Input
                    id="installCommand"
                    value={installCommand}
                    onChange={(e) => setInstallCommand(e.target.value)}
                    placeholder="npm install"
                    className="border-white/10 bg-white/[0.03] font-mono text-xs text-white"
                  />
                </div>
              </div>

              <div className="rounded-xl border border-white/[0.08] bg-white/[0.02] p-4">
                <div className="flex items-center gap-2 text-xs font-medium">
                  <Zap className="size-3.5 text-cyan-300" />
                  <span>Framework: {preset?.displayName ?? "Unknown"}</span>
                </div>
                <dl className="mt-3 grid grid-cols-2 gap-2 text-[10px] text-white/40">
                  <dt>Build</dt><dd className="font-mono text-right text-white/60">{preset?.buildCommand ?? "—"}</dd>
                  <dt>Output</dt><dd className="font-mono text-right text-white/60">{preset?.outputDirectory ?? "—"}</dd>
                  <dt>Install</dt><dd className="font-mono text-right text-white/60">{preset?.installCommand ?? "—"}</dd>
                  <dt>Dev</dt><dd className="font-mono text-right text-white/60">{preset?.devCommand ?? "—"}</dd>
                </dl>
              </div>
            </div>
          )}

          {step === "env" && (
            <div className="space-y-6">
              <EnvVarTable
                variables={envVars}
                onChange={setEnvVars}
                placeholderKey="NEXT_PUBLIC_API_URL"
                placeholderValue="https://api.example.com"
              />
              <Separator />
              <EnvVarImport onImport={(vars) => {
                setEnvVars(Object.entries(vars).map(([key, value]) => ({
                  key,
                  value,
                  isSecret: !key.startsWith("NEXT_PUBLIC_") && !key.startsWith("VITE_"),
                })));
              }} />
            </div>
          )}

          {step === "deploy" && (
            <div className="space-y-6">
              <div className="rounded-xl border border-white/[0.08] bg-white/[0.02] p-5">
                <h4 className="text-sm font-medium">Review Configuration</h4>
                <dl className="mt-4 space-y-3 text-sm">
                  <div className="flex justify-between gap-4">
                    <dt className="text-white/30">Repository</dt>
                    <dd className="font-mono text-right text-white/60 truncate max-w-[60%]">
                      {selectedRepo?.full_name ?? "—"}
                    </dd>
                  </div>
                  <div className="flex justify-between gap-4">
                    <dt className="text-white/30">Branch</dt>
                    <dd className="font-mono text-right text-white/60">{branch || selectedRepo?.default_branch}</dd>
                  </div>
                  <div className="flex justify-between gap-4">
                    <dt className="text-white/30">Framework</dt>
                    <dd className="text-right">
                      <FrameworkBadge framework={framework} size="sm" />
                    </dd>
                  </div>
                  <div className="flex justify-between gap-4">
                    <dt className="text-white/30">Build Command</dt>
                    <dd className="font-mono text-right text-white/60 truncate max-w-[60%]">{buildCommand}</dd>
                  </div>
                  <div className="flex justify-between gap-4">
                    <dt className="text-white/30">Output Directory</dt>
                    <dd className="font-mono text-right text-white/60">{outputDir}</dd>
                  </div>
                  <div className="flex justify-between gap-4">
                    <dt className="text-white/30">Environment Variables</dt>
                    <dd className="text-right text-white/60">{envVars.length} configured</dd>
                  </div>
                </dl>
              </div>

              {create.isError && (
                <div className="rounded-xl border border-rose-400/30 bg-rose-400/5 p-4 text-sm text-rose-200/70">
                  {create.error?.message ?? "Failed to create project"}
                </div>
              )}

              <div className="rounded-xl border border-cyan-400/20 bg-cyan-400/5 p-4">
                <p className="text-xs text-cyan-200/70">
                  This will create a new project and trigger the first build. 
                  The build will run on our workers and you can monitor progress on the project page.
                </p>
              </div>
            </div>
          )}
        </div>

        <div className="flex-shrink-0 flex items-center justify-between gap-2 border-t border-white/[0.07] px-6 py-4">
          <Button
            variant="ghost"
            onClick={goBack}
            disabled={currentStepIndex === 0 || create.isPending || isDeploying}
          >
            <ChevronLeft className="size-3.5 mr-1.5" /> Back
          </Button>
          <div className="flex-1" />
          {step === "deploy" ? (
            <Button
              onClick={handleDeploy}
              disabled={!selectedRepo || !branch.trim() || !buildCommand.trim() || create.isPending || isDeploying}
              className="gap-2"
            >
              {isDeploying || create.isPending ? (
                <>
                  <Loader2 className="size-3.5 animate-spin" />
                  Creating project…
                </>
              ) : (
                <>
                  <Rocket className="size-3.5" />
                  Create & Deploy
                </>
              )}
            </Button>
          ) : (
            <Button
              onClick={goNext}
              disabled={step === "configure" && (!branch.trim() || !buildCommand.trim())}
              className="gap-2"
            >
              Next <ChevronRight className="size-3.5" />
            </Button>
          )}
        </div>
      </DialogContent>
    </Dialog>
  );
}