import type {
  Transaction,
  User,
  Tag,
  TransactionSplit,
  Statistics,
  MonthlySpending,
  TagSpending,
  ReminderRequest,
  BillSplitRequest,
  UserSplitSummary,
  ApiResponse,
} from "$lib/types";

const API_BASE_URL =
  import.meta.env.VITE_API_URL || "http://localhost:8080/api";

class ApiClient {
  private async request<T>(
    endpoint: string,
    options: RequestInit = {}
  ): Promise<ApiResponse<T>> {
    const url = `${API_BASE_URL}${endpoint}`;

    try {
      const response = await fetch(url, {
        ...options,
        headers: {
          "Content-Type": "application/json",
          ...options.headers,
        },
      });

      if (!response.ok) {
        const error = await response
          .json()
          .catch(() => ({ error: response.statusText }));
        return { error: error.error || "Request failed" };
      }

      const data = await response.json();
      return data;
    } catch (error) {
      return {
        error: error instanceof Error ? error.message : "Unknown error",
      };
    }
  }

  // Transactions
  async getTransactions(
    limit = 20,
    offset = 0
  ): Promise<ApiResponse<Transaction[]>> {
    return this.request<Transaction[]>(
      `/transactions?limit=${limit}&offset=${offset}`
    );
  }

  async getTransaction(id: number): Promise<ApiResponse<Transaction>> {
    return this.request<Transaction>(`/transactions/${id}`);
  }

  async createVirtualBill(
    amount: number,
    description: string
  ): Promise<ApiResponse<Transaction>> {
    return this.request<Transaction>("/transactions/virtual", {
      method: "POST",
      body: JSON.stringify({ amount, description }),
    });
  }

  async deleteTransaction(id: number): Promise<ApiResponse<void>> {
    return this.request<void>(`/transactions/${id}`, {
      method: "DELETE",
    });
  }

  // Users
  async getUsers(): Promise<ApiResponse<User[]>> {
    return this.request<User[]>("/users");
  }

  async getUser(id: number): Promise<ApiResponse<User>> {
    return this.request<User>(`/users/${id}`);
  }

  async createUser(
    user: Omit<User, "id" | "created_at">
  ): Promise<ApiResponse<User>> {
    return this.request<User>("/users", {
      method: "POST",
      body: JSON.stringify(user),
    });
  }

  async updateUser(
    id: number,
    user: Partial<User>
  ): Promise<ApiResponse<User>> {
    return this.request<User>(`/users/${id}`, {
      method: "PUT",
      body: JSON.stringify(user),
    });
  }

  async deleteUser(id: number): Promise<ApiResponse<void>> {
    return this.request<void>(`/users/${id}`, {
      method: "DELETE",
    });
  }

  // Tags
  async getTags(): Promise<ApiResponse<Tag[]>> {
    return this.request<Tag[]>("/tags");
  }

  async createTag(name: string): Promise<ApiResponse<Tag>> {
    return this.request<Tag>("/tags", {
      method: "POST",
      body: JSON.stringify({ name }),
    });
  }

  async addTagToTransaction(
    transactionId: number,
    tagId: number
  ): Promise<ApiResponse<void>> {
    return this.request<void>(`/transactions/${transactionId}/tags/${tagId}`, {
      method: "POST",
    });
  }

  async removeTagFromTransaction(
    transactionId: number,
    tagId: number
  ): Promise<ApiResponse<void>> {
    return this.request<void>(`/transactions/${transactionId}/tags/${tagId}`, {
      method: "DELETE",
    });
  }

  // Bill Splitting
  async createBillSplit(
    request: BillSplitRequest
  ): Promise<ApiResponse<TransactionSplit[]>> {
    return this.request<TransactionSplit[]>("/splits", {
      method: "POST",
      body: JSON.stringify(request),
    });
  }

  async getSplitsForTransaction(
    transactionId: number
  ): Promise<ApiResponse<TransactionSplit[]>> {
    return this.request<TransactionSplit[]>(
      `/transactions/${transactionId}/splits`
    );
  }

  async completeSplit(splitId: number): Promise<ApiResponse<void>> {
    return this.request<void>(`/splits/${splitId}/complete`, {
      method: "POST",
    });
  }

  async deleteSplit(splitId: number): Promise<ApiResponse<void>> {
    return this.request<void>(`/splits/${splitId}`, {
      method: "DELETE",
    });
  }

  async completeTransaction(transactionId: number): Promise<ApiResponse<void>> {
    return this.request<void>(`/transactions/${transactionId}/complete`, {
      method: "POST",
    });
  }

  // Reminders & Splits
  async getPendingSplits(): Promise<ApiResponse<UserSplitSummary[]>> {
    return this.request<UserSplitSummary[]>("/splits/pending");
  }

  async sendReminder(request: ReminderRequest): Promise<ApiResponse<void>> {
    return this.request<void>("/reminders", {
      method: "POST",
      body: JSON.stringify(request),
    });
  }

  // Statistics
  async getStatistics(): Promise<ApiResponse<Statistics>> {
    return this.request<Statistics>("/statistics");
  }

  async getSpendingByMonth(year?: number): Promise<ApiResponse<{ months: MonthlySpending[] }>> {
    const url = year
      ? `/statistics/monthly?year=${year}`
      : "/statistics/monthly";
    return this.request<{ months: MonthlySpending[] }>(url);
  }

  async getSpendingByTag(): Promise<ApiResponse<{ tags: TagSpending[] }>> {
    return this.request<{ tags: TagSpending[] }>("/statistics/tags");
  }
}

export const api = new ApiClient();
