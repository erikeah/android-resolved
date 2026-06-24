package com.androidresolved;

interface IVpnControlService {
    String start();
    void stop();
    boolean isRunning();
    String getStatus();
    String getStats();
    String getVersion();
    void addRule(String ruleJson);
    void flushRules();
}
